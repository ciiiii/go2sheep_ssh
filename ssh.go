package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSH struct {
	*ssh.Client
	info    *Info
	bastion *SSH
}

func New(info *Info) *SSH {
	return &SSH{
		info: info,
	}
}

func (s *SSH) Bastion(info *Info) *SSH {
	s.bastion = New(info)
	return s
}

func (s *SSH) Close() {
	_ = s.Client.Close()
	if s.bastion != nil {
		s.bastion.Close()
	}
}

func (s *SSH) Connect() (*SSH, error) {
	config, err := s.info.prepare()
	if err != nil {
		return nil, err
	}
	if s.bastion != nil {
		if _, err := s.bastion.Connect(); err != nil {
			return nil, err
		}
		tunnelConn, err := s.bastion.Dial("tcp", s.bastion.info.serverAddr())
		if err != nil {
			return nil, err
		}
		conn, chans, reqs, err := ssh.NewClientConn(tunnelConn, s.bastion.info.serverAddr(), config)
		if err != nil {
			return nil, err
		}
		client := ssh.NewClient(conn, chans, reqs)
		s.Client = client
		fmt.Printf("[SSH] connected to %s over %s..\n", s.info.serverAddr(), s.bastion.info.serverAddr())
	} else {
		client, err := ssh.Dial("tcp", s.info.serverAddr(), config)
		if err != nil {
			return nil, err
		}
		s.Client = client
		fmt.Printf("[SSH] connected to %s..\n", s.info.serverAddr())
	}
	return s, err
}

func (s *SSH) ExecCmd(cmd string) (string, string, error) {
	session, err := s.NewSession()
	if err != nil {
		return "", "", err
	}
	defer func() {
		_ = session.Close()
	}()
	var stdOut bytes.Buffer
	var stdErr bytes.Buffer

	session.Stdout = &stdOut
	session.Stderr = &stdErr

	if err := session.Run(cmd); err != nil {
		return stdOut.String(), stdErr.String(), err
	}
	return stdOut.String(), stdErr.String(), nil
}

func (s *SSH) ExecCmdPipe(cmd string, output chan string) error {
	session, err := s.NewSession()
	if err != nil {
		return err
	}
	defer func() {
		_ = session.Close()
	}()
	stdout, err := session.StdoutPipe()
	if err != nil {
		return err
	}
	if err := session.Start(cmd); err != nil {
		return err
	}
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil || err == io.EOF {
			break
		}
		output <- line
	}
	return session.Wait()
}

func (s *SSH) ProxyHttpTransport() http.RoundTripper {
	return &http.Transport{
		Dial:                s.Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

func (s *SSH) ProxyHttpClient() *http.Client {
	client := http.Client{
		Transport: s.ProxyHttpTransport(),
	}
	return &client
}

func (s *SSH) TunnelProxy(localHost, remoteHost string, localPort, remotePort int, closeCh chan os.Signal) error {
	localAddr := fmt.Sprintf("%s:%d", localHost, localPort)
	remoateAddr := fmt.Sprintf("%s:%d", remoteHost, remotePort)
	localLister, err := net.Listen("tcp", localAddr)
	if err != nil {
		return err
	}
	defer localLister.Close()
	fmt.Printf("[TUNNEL] start listening on %s...\n", localAddr)

	errCh := make(chan error)
	go func() {
		for {
			local, err := localLister.Accept()
			if err != nil {
				if err == io.EOF {
					continue
				}
				break
			}
			go s.handleClient(local, remoateAddr, errCh)
		}
	}()
	for {
		select {
		case <-closeCh:
			fmt.Printf("[TUNNEL] stop listening on %s:%d...\n", localHost, localPort)
			return nil
		case err := <-errCh:
			return err
		}
	}
}

func (s *SSH) handleClient(local net.Conn, remoteAddr string, errCh chan error) {
	remote, err := s.Dial("tcp", remoteAddr)
	if err != nil {
		errCh <- err
		return
	}
	fmt.Printf("[TUNNEL] request %s over %s..\n", remoteAddr, s.info.serverAddr())
	defer remote.Close()
	defer local.Close()

	exchangeErrCh := make(chan error, 1)
	go exchange(remote, local, exchangeErrCh)
	go exchange(local, remote, exchangeErrCh)

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			if err == io.EOF {
				fmt.Println("[TUNNEL] request EOF")
			}
			if _, ok := err.(*net.OpError); !ok {

			}
		}
	}
}
