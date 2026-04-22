package tunnel

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	"github.com/pkg/errors"
	"github.com/tianjinli/dragz/pkg/appkit"
	"golang.org/x/crypto/ssh"
)

type SshExpose struct {
	conn      *ssh.Client
	listener  net.Listener
	localAddr string
}

func NewSshExpose(conf *appkit.TunnelConfig, num uint16) (*SshExpose, error) {
	if conf == nil || conf.Scheme != "ssh" {
		return nil, nil
	}
	conn, err := appkit.NewSshTunnel(conf)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var port = fmt.Sprintf("%d", num)
	listener, err := conn.Listen("tcp", net.JoinHostPort("0.0.0.0", port))
	if err != nil {
		return nil, errors.WithStack(err)
	}
	localAddr := net.JoinHostPort("127.0.0.1", port)
	expose := &SshExpose{
		conn:      conn,
		listener:  listener,
		localAddr: localAddr,
	}
	log.Println("ssh tunnel started: remote", listener.Addr().String(), "-> local", localAddr)
	go expose.remoteAcceptLoop()
	return expose, nil
}

func (s *SshExpose) remoteAcceptLoop() {
	for {
		remoteConn, err := s.listener.Accept()
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			break
		}
		go s.handleConnection(remoteConn, s.localAddr)
	}
}

func (s *SshExpose) handleConnection(remoteConn net.Conn, localAddr string) {
	defer func(remoteConn net.Conn) {
		_ = remoteConn.Close()
	}(remoteConn)
	localConn, err := net.Dial("tcp", localAddr)
	if err != nil {
		return
	}
	defer func(localConn net.Conn) {
		_ = localConn.Close()
	}(localConn)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(localConn, remoteConn)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Printf("remote -> local: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(remoteConn, localConn)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Printf("local -> remote: %v", err)
		}
	}()

	wg.Wait()
	log.Printf("<- Connection closed: %s", remoteConn.RemoteAddr())
}

func (s *SshExpose) Addr() net.Addr {
	return s.listener.Addr()
}

func (s *SshExpose) Close() error {
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
