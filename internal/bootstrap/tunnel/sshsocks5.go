package tunnel

import (
	"log"
	"net"
	"os"

	"github.com/armon/go-socks5"
	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/pkg/appkit"
	"golang.org/x/crypto/ssh"
)

type SshSocks5 struct {
	conn     *ssh.Client
	listener net.Listener
}

func NewSshSocks5(conf *appkit.TunnelConfig) (*SshSocks5, error) {
	if conf == nil || conf.Scheme != "ssh" {
		return nil, nil
	}
	conn, err := appkit.NewSshTunnel(conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var host = "127.0.0.1"
	var port = "0"
	listener, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, errors.WithStack(err)
	}

	server, err := socks5.New(&socks5.Config{
		Dial:   conn.DialContext,
		Logger: log.New(os.Stderr, "[SOCKS5] ", log.LstdFlags),
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	go func() {
		if err = server.Serve(listener); err != nil {
			log.Printf("[SOCKS5] Serve error: %v", err)
		}
	}()

	return &SshSocks5{
		conn:     conn,
		listener: listener,
	}, nil
}

func (s *SshSocks5) Addr() net.Addr {
	if s.listener == nil {
		return nil
	}
	return s.listener.Addr()
}

func (s *SshSocks5) Close() error {
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			return errors.WithStack(err)
		}
	}
	if s.conn != nil {
		if err := s.conn.Close(); err != nil {
			return errors.WithStack(err)
		}
	}
	return nil
}
