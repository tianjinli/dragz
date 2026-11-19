package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type recordingPublisher struct {
	mu     sync.Mutex
	bodies [][]byte
}

func (p *recordingPublisher) Publish(_ context.Context, _ string, payload []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bodies = append(p.bodies, append([]byte(nil), payload...))
	return nil
}

func TestHubPushDeliversLocallyAndSkipsOwnOrigin(t *testing.T) {
	hub := newHub(nil)
	published := &recordingPublisher{}
	hub.remote = published
	defer hub.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleHTTP(w, r, 7)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	incoming := make(chan []byte, 4)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			incoming <- msg
		}
	}()

	payload := map[string]any{"type": "notice", "id": float64(3)}
	if err := hub.Push([]uint64{7, 7, 0}, payload); err != nil {
		t.Fatalf("Push: %v", err)
	}
	want, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	got := readOne(t, incoming)
	if string(got) != string(want) {
		t.Fatalf("socket message = %s, want %s", got, want)
	}

	published.mu.Lock()
	if len(published.bodies) != 1 {
		published.mu.Unlock()
		t.Fatalf("published %d envelopes, want 1", len(published.bodies))
	}
	raw := append([]byte(nil), published.bodies[0]...)
	published.mu.Unlock()

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("envelope: %v", err)
	}
	if env.Origin != hub.origin {
		t.Fatalf("origin = %q, want %q", env.Origin, hub.origin)
	}
	if len(env.UserIDs) != 1 || env.UserIDs[0] != 7 {
		t.Fatalf("userIds = %v, want [7]", env.UserIDs)
	}

	hub.accept(env)
	expectQuiet(t, incoming)

	env.Origin = "other-process"
	hub.accept(env)
	got = readOne(t, incoming)
	if string(got) != string(want) {
		t.Fatalf("foreign envelope = %s, want %s", got, want)
	}

	conn.Close()
	select {
	case <-readDone:
	case <-time.After(2 * time.Second):
		t.Fatal("reader did not exit")
	}
}

func readOne(t *testing.T, incoming <-chan []byte) []byte {
	t.Helper()
	select {
	case msg, ok := <-incoming:
		if !ok {
			t.Fatal("socket closed before a message")
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a socket message")
		return nil
	}
}

func expectQuiet(t *testing.T, incoming <-chan []byte) {
	t.Helper()
	select {
	case msg := <-incoming:
		t.Fatalf("own origin was delivered again: %s", msg)
	case <-time.After(150 * time.Millisecond):
	}
}
