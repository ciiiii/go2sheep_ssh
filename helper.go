package ssh

import (
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

type Info struct {
	Host     string
	Username string
	Port     int
	Password string
	Key      string
	Timeout  int
}

func (info *Info) prepare() (*ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod
	if info.Key != "" {
		signer, err := ssh.ParsePrivateKey([]byte(info.Key))
		if err != nil {
			return nil, err
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	if info.Password != "" {
		auth = append(auth, ssh.Password(info.Password))
	}
	config := ssh.ClientConfig{
		User:            info.Username,
		Auth:            auth,
		HostKeyCallback: ssh.HostKeyCallback(func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil }),
		Timeout:         time.Duration(time.Second.Nanoseconds() * int64(info.Timeout)),
	}
	return &config, nil
}

func (info *Info) serverAddr() string {
	return fmt.Sprintf("%s:%d", info.Host, info.Port)
}

type closeWriter interface {
	CloseWrite() error
}

func exchange(dst io.Writer, src io.Reader, errCh chan error) {
	_, err := io.Copy(dst, src)
	if tcpConn, ok := dst.(closeWriter); ok {
		tcpConn.CloseWrite()
	}
	errCh <- err
}
