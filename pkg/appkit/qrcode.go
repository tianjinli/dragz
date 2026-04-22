package appkit

import (
	"net/http"
	"time"
)

const (
	WebsocketWriteTimeout = 10 * time.Second
)

type QrcodeService interface {
	HandleRequest(w http.ResponseWriter, r *http.Request) error
	HandleLogin(uniqueID string) error
}
