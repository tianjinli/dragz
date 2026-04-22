package appkit

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// _ automatically adds unknown hosts to the known_hosts file. (not recommended for production)
func _(knownHosts string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if _, err := os.Stat(knownHosts); os.IsNotExist(err) {
			if err = os.MkdirAll(filepath.Dir(knownHosts), 0700); err != nil {
				return err
			}
			file, err := os.OpenFile(knownHosts, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			_ = file.Close()
		}
		callback, err := knownhosts.New(knownHosts)
		if err != nil {
			return err
		}
		if err = callback(hostname, remote, key); err != nil {
			var keyError *knownhosts.KeyError
			if !errors.As(err, &keyError) {
				return err
			}
			file, err := os.OpenFile(knownHosts, os.O_APPEND|os.O_WRONLY, os.ModePerm)
			if err != nil {
				return err
			}
			defer func() { _ = file.Close() }()

			line := fmt.Sprintln(knownhosts.Line([]string{remote.String()}, key))
			if _, err = file.WriteString(line); err != nil {
				return err
			}
			callback, err = knownhosts.New(knownHosts)
			if err != nil {
				return err
			}
			return callback(hostname, remote, key)
		}
		return nil
	}
}

func NewSshTunnel(conf *TunnelConfig) (*ssh.Client, error) {
	var hostKeyCallback ssh.HostKeyCallback
	var userHome string
	if runtime.GOOS != "windows" {
		userHome = os.Getenv("HOME")
	} else {
		userHome = os.Getenv("USERPROFILE")
	}
	if Debug {
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		knownHosts := filepath.Join(userHome, ".ssh", "known_hosts")
		if conf.KnownHosts != "" {
			if strings.HasPrefix(conf.KnownHosts, "~/") {
				knownHosts = filepath.Join(userHome, conf.KnownHosts[2:])
			} else {
				knownHosts = conf.KnownHosts
			}
		}
		hostKeyCallback, _ = knownhosts.New(knownHosts)
	}

	var authMethods []ssh.AuthMethod
	if conf.Password != "" {
		authMethods = append(authMethods, ssh.Password(conf.Password))
	}
	if conf.PrivateKey != "" {
		var privateKey string
		if strings.HasPrefix(conf.PrivateKey, "~/") {
			elem := conf.PrivateKey[2:]
			if elem == "" {
				elem = "id_rsa"
			}
			privateKey = filepath.Join(userHome, elem)
		} else {
			privateKey = conf.PrivateKey
		}
		key, err := os.ReadFile(privateKey)
		if err != nil {
			return nil, err
		}
		var signer ssh.Signer
		if conf.Passphrase == "" {
			signer, err = ssh.ParsePrivateKey(key)
		} else {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(conf.Passphrase))
		}
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	addr := fmt.Sprintf("%s:%d", conf.Host, conf.Port)
	client, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{
		User:            conf.Username,
		Auth:            authMethods,
		Timeout:         conf.Timeout,
		HostKeyCallback: hostKeyCallback,
	})
	defer func() {
		if err != nil && client != nil {
			_ = client.Close()
		}
	}()
	if err != nil {
		return nil, err
	}
	return client, nil
}
