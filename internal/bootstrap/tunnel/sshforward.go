package tunnel

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/pkg/appkit"
	"golang.org/x/crypto/ssh"
)

type SshForward struct {
	conn       *ssh.Client
	listener   net.Listener
	remoteChan chan net.Conn
}

func NewSshForward(conf *appkit.TunnelConfig) (*SshForward, error) {
	if conf == nil || conf.Scheme != "ssh" {
		return nil, nil
	}
	conn, err := appkit.NewSshTunnel(conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var host = "0.0.0.0"
	var port = "0"
	listener, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	proxy := &SshForward{
		conn:       conn,
		listener:   listener,
		remoteChan: make(chan net.Conn, 1), // to prevent remoteChan being blocked
	}
	go proxy.forwardLoop()
	return proxy, nil
}

func (s *SshForward) forwardLoop() {
	var remoteConn net.Conn
	for {
		select {
		case remoteConn = <-s.remoteChan:
		}

		localConn, err := s.listener.Accept()
		if err != nil {
			return
		}
		if remoteConn == nil {
			_ = localConn.Close()
			continue
		}
		go func() {
			defer func(localConn net.Conn) {
				_ = localConn.Close()
			}(localConn)
			defer func(remoteConn net.Conn) {
				_ = remoteConn.Close()
			}(remoteConn)
			_, _ = io.Copy(remoteConn, localConn)
		}()
		go func() {
			defer func(localConn net.Conn) {
				_ = localConn.Close()
			}(localConn)
			defer func(remoteConn net.Conn) {
				_ = remoteConn.Close()
			}(remoteConn)
			_, _ = io.Copy(localConn, remoteConn)
		}()
	}
}

func (s *SshForward) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	remoteConn, err := s.conn.DialContext(ctx, network, addr)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	select {
	case s.remoteChan <- remoteConn:
	case <-ctx.Done():
		return nil, errors.WithStack(ctx.Err())
	}
	log.Println("ssh tunnel established: local", s.listener.Addr().String(), "-> remote", addr)
	return net.Dial(network, s.listener.Addr().String())
}

func (s *SshForward) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *SshForward) Close() error {
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
