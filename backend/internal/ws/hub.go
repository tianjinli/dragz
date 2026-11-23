// Package ws delivers realtime events to connected admin clients.
// Each process keeps the sockets it upgraded. Push writes those sockets and
// publishes the same payload on Redis so other processes can deliver it.
// A subscriber drops envelopes it published itself, because Push already
// delivered them locally.
package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	redisChannel = "dragz:ws"
	writeWait    = 5 * time.Second
	pongWait     = 60 * time.Second
	pingPeriod   = 30 * time.Second
)

// envelope is the Redis message. Payload is the event already encoded for the socket.
// TenantID scopes the fan-out: a socket belongs to exactly one tenant and must
// never receive another tenant's event, even when the user id matches.
type envelope struct {
	Origin   string          `json:"origin"`
	TenantID uint64          `json:"tenantId"`
	UserIDs  []uint64        `json:"userIds"`
	Payload  json.RawMessage `json:"payload"`
}

type publisher interface {
	Publish(ctx context.Context, channel string, payload []byte) error
}

type redisPublisher struct {
	client *redis.Client
}

func (p redisPublisher) Publish(ctx context.Context, channel string, payload []byte) error {
	return p.client.Publish(ctx, channel, payload).Err()
}

// peer is one upgraded socket. gorilla allows one concurrent reader and one
// concurrent writer, so every write takes mu.
// tenantID is the tenant the socket authenticated into; it is fixed for the
// lifetime of the connection and gates every delivery.
type peer struct {
	userID   uint64
	tenantID uint64
	conn     *websocket.Conn
	mu       sync.Mutex
}

func (p *peer) write(messageType int, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return p.conn.WriteMessage(messageType, data)
}

// Hub is the process-local socket table plus the Redis fan-out.
type Hub struct {
	origin string
	client *redis.Client
	remote publisher

	mu     sync.RWMutex
	conns  map[uint64]map[*peer]struct{}
	closed bool

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

// upgrader accepts any Origin. The socket is authenticated with the access
// token before Upgrade, and the admin UI is served from a different origin
// than the API in development.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

// NewHub connects to Redis at addr and starts the subscriber.
// password may be empty. Host is a go-redis address such as "127.0.0.1:6379".
func NewHub(addr, password string) (*Hub, error) {
	if addr == "" {
		return nil, fmt.Errorf("redis address is empty")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, Password: password})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	err := client.Ping(ctx).Err()
	cancel()
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	h := newHub(client)
	h.remote = redisPublisher{client: client}
	go h.run()
	return h, nil
}

func newHub(client *redis.Client) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		origin: uuid.NewString(),
		client: client,
		conns:  map[uint64]map[*peer]struct{}{},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Push delivers payload to every local socket of userIDs inside tenantID and
// publishes the same event for other processes. A nil hub, a zero tenant, or an
// empty audience is a no-op: a realtime event without a tenant would be a
// cross-tenant broadcast, so it is refused rather than guessed.
func (h *Hub) Push(tenantID uint64, userIDs []uint64, payload any) error {
	if h == nil || tenantID == 0 {
		return nil
	}
	ids := uniqueUserIDs(userIDs)
	if len(ids) == 0 {
		return nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	h.deliver(tenantID, ids, raw)
	body, err := json.Marshal(envelope{Origin: h.origin, TenantID: tenantID, UserIDs: ids, Payload: raw})
	if err != nil {
		return err
	}
	if h.remote == nil {
		return nil
	}
	if err := h.remote.Publish(context.Background(), redisChannel, body); err != nil {
		return fmt.Errorf("publish ws event: %w", err)
	}
	return nil
}

// HandleHTTP upgrades the request and keeps the socket until it closes.
// userID is the authenticated principal and tenantID is the tenant the token is
// scoped to; the caller has already checked both.
func (h *Hub) HandleHTTP(w http.ResponseWriter, r *http.Request, tenantID, userID uint64) {
	if h == nil || userID == 0 || tenantID == 0 {
		http.Error(w, "realtime channel unavailable", http.StatusServiceUnavailable)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Errorf("ws upgrade: %v", err)
		return
	}
	p := &peer{userID: userID, tenantID: tenantID, conn: conn}
	if !h.add(p) {
		_ = conn.Close()
		return
	}
	defer h.remove(p)
	defer conn.Close()

	done := make(chan struct{})
	defer close(done)
	go h.ping(p, done)

	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	conn.SetReadLimit(1 << 16)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// Close stops the subscriber and drops every local socket.
func (h *Hub) Close() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		h.cancel()
		h.mu.Lock()
		h.closed = true
		for _, set := range h.conns {
			for p := range set {
				_ = p.conn.Close()
			}
		}
		h.conns = map[uint64]map[*peer]struct{}{}
		client := h.client
		h.mu.Unlock()
		if client != nil {
			_ = client.Close()
		}
	})
}

func (h *Hub) ping(p *peer, done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			if err := p.write(websocket.PingMessage, nil); err != nil {
				_ = p.conn.Close()
				return
			}
		}
	}
}

func (h *Hub) run() {
	for h.ctx.Err() == nil {
		if !h.consume() {
			return
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-h.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (h *Hub) consume() bool {
	pubsub := h.client.Subscribe(h.ctx, redisChannel)
	defer pubsub.Close()
	ch := pubsub.Channel()
	for {
		select {
		case <-h.ctx.Done():
			return false
		case msg, ok := <-ch:
			if !ok {
				return true
			}
			h.acceptRaw(msg.Payload)
		}
	}
}

func (h *Hub) acceptRaw(payload string) {
	var env envelope
	if err := json.Unmarshal([]byte(payload), &env); err != nil {
		return
	}
	h.accept(env)
}

func (h *Hub) accept(env envelope) {
	if env.Origin == h.origin || env.TenantID == 0 || len(env.UserIDs) == 0 || len(env.Payload) == 0 {
		return
	}
	h.deliver(env.TenantID, env.UserIDs, env.Payload)
}

func (h *Hub) deliver(tenantID uint64, userIDs []uint64, payload []byte) {
	h.mu.RLock()
	targets := make([]*peer, 0, len(userIDs))
	for _, id := range userIDs {
		for p := range h.conns[id] {
			if p.tenantID == tenantID {
				targets = append(targets, p)
			}
		}
	}
	h.mu.RUnlock()
	for _, p := range targets {
		if err := p.write(websocket.TextMessage, payload); err != nil {
			h.remove(p)
			_ = p.conn.Close()
		}
	}
}

func (h *Hub) add(p *peer) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}
	set := h.conns[p.userID]
	if set == nil {
		set = map[*peer]struct{}{}
		h.conns[p.userID] = set
	}
	set[p] = struct{}{}
	return true
}

func (h *Hub) remove(p *peer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.conns[p.userID]
	delete(set, p)
	if len(set) == 0 {
		delete(h.conns, p.userID)
	}
}

func uniqueUserIDs(ids []uint64) []uint64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(ids))
	out := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}
