package qrcode

import (
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/olahol/melody"
	"github.com/tianjinli/dragz/internal/i18n"
	"github.com/tianjinli/dragz/pkg/appkit"
	"go.uber.org/zap"
)

const qrcodeUniqueKey = "ws.qrcode.uid"

// qrcodeScanTimeout is QR code scanning timed out
const qrcodeScanTimeout = 60 * time.Second

type qrcodeService struct {
	logger *zap.Logger

	manager  *melody.Melody `wire:"-"`
	sessions sync.Map       `wire:"-"` // key: userID, value: *melody.Session
}

func NewQrcodeService(logger *zap.Logger) appkit.QrcodeService {
	return (&qrcodeService{logger: logger}).init()
}

func (q *qrcodeService) init() *qrcodeService {
	q.manager = melody.New()
	q.manager.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	q.manager.HandleConnect(q.handleConnect)
	q.manager.HandleDisconnect(q.handleDisconnect)
	return q
}

func (q *qrcodeService) handleConnect(s *melody.Session) {
	uniqueID := s.MustGet(qrcodeUniqueKey).(string)

	// if the user already has a connection, close it
	if old, ok := q.sessions.Load(uniqueID); ok {
		if oldSession, ok2 := old.(*melody.Session); ok2 {
			_ = oldSession.Close() // close the old connection
		}
		//q.sessions.Delete(uniqueID)
	}
	uri := s.Request.RequestURI
	if uri != "" && uri[len(uri)-1:] != "/" {
		uri += "/"
	}
	err := s.WriteWithDeadline([]byte(uri+uniqueID), appkit.WebsocketWriteTimeout)
	if err != nil {
		q.logger.Error("websocket write", zap.String("unique-id", uniqueID), zap.Error(err))
	}
	time.AfterFunc(qrcodeScanTimeout, func() {
		if err = s.Close(); err != nil {
			q.logger.Error("websocket close", zap.String("unique-id", uniqueID), zap.Error(err))
		}
	})
	// bind the new connection
	q.sessions.Store(uniqueID, s)
}

func (q *qrcodeService) handleDisconnect(s *melody.Session) {
	uniqueID := s.MustGet(qrcodeUniqueKey).(string)
	if stored, ok := q.sessions.Load(uniqueID); ok && stored == s {
		q.sessions.Delete(uniqueID)
	}
}

func (q *qrcodeService) HandleRequest(w http.ResponseWriter, r *http.Request) error {
	uniqueID := uuid.New().String()
	err := q.manager.HandleRequestWithKeys(w, r, map[string]any{qrcodeUniqueKey: uniqueID}) // no error will block until connection close
	if err != nil {
		q.logger.Error("websocket handle", zap.String("unique-id", uniqueID), zap.Error(err))
		return appkit.NewInternalServerError(i18n.ErrBaseInvalidParams).WithError(err)
	}
	return nil
}

func (q *qrcodeService) HandleLogin(uniqueID string) error {
	stored, ok := q.sessions.Load(uniqueID)
	if !ok {
		return appkit.NewRequestTimeout(i18n.ErrBaseQrcodeExpired)
	}
	if session, ok := stored.(*melody.Session); ok {
		if err := session.Close(); err != nil {
			q.logger.Error("websocket close", zap.String("unique-id", uniqueID), zap.Error(err))
		}
		return nil
	}
	return appkit.NewInternalServerError(i18n.ErrBaseUnknownError)
}
