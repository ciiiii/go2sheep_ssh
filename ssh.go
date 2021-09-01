package ssh

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSH struct {
	ssh     *ssh.Client
	bastion *ssh.Client
}

type Info struct {
	Host     string
	Username string
	Port     int
	Password string
	Key      string
	Timeout  int
}

func (info Info) prepare() (string, *ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod
	if info.Key != "" {
		signer, err := ssh.ParsePrivateKey([]byte(info.Key))
		if err != nil {
			return "", nil, err
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
	return fmt.Sprintf("%s:%d", info.Host, info.Port), &config, nil
}

func NewSSHClient(info Info) (*SSH, error) {
	serverAddr, config, err := info.prepare()
	if err != nil {
		return nil, err
	}
	client, err := ssh.Dial("tcp", serverAddr, config)
	if err != nil {
		return nil, err
	}
	return &SSH{
		ssh:     client,
		bastion: client,
	}, nil
}

func NewSSHClientWithBastion(bastionInfo, info Info) (*SSH, error) {
	bastionAddr, bastionConfig, err := bastionInfo.prepare()
	if err != nil {
		return nil, err
	}
	bastionClient, err := ssh.Dial("tcp", bastionAddr, bastionConfig)
	if err != nil {
		return nil, err
	}

	serverAddr, serverConfig, err := info.prepare()
	if err != nil {
		return nil, err
	}

	tunnelConn, err := bastionClient.Dial("tcp", serverAddr)
	if err != nil {
		return nil, err
	}

	conn, chans, reqs, err := ssh.NewClientConn(tunnelConn, serverAddr, serverConfig)
	if err != nil {
		return nil, err
	}
	client := ssh.NewClient(conn, chans, reqs)

	return &SSH{
		ssh:     client,
		bastion: bastionClient,
	}, nil
}

func (c *SSH) TunnelProxy(remoteHost string, remotePort, localPort int, closeCh chan bool) {
	local, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		fmt.Println("[SSH]", err)
	}
	defer local.Close()
	fmt.Printf("[SSH] start listening on %d...\n", localPort)

	go func() {
		for {
			client, err := local.Accept()
			if err != nil {
				if err == io.EOF {
					continue
				}
				break
			}
			go c.handleClient(client, remoteHost, remotePort)
		}
		fmt.Printf("[SSH] stop listening on %d...\n", localPort)
	}()
	<-closeCh
}

func (c *SSH) handleClient(client net.Conn, remoteHost string, remotePort int) {
	remote, err := c.bastion.Dial("tcp", fmt.Sprintf("%s:%d", remoteHost, remotePort))
	if err != nil {
		panic(err)
	}
	defer remote.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go exchange(remote, client, errCh)
	go exchange(client, remote, errCh)

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			if err == io.EOF {
				fmt.Println("[SSH] EOF")
			}
			if _, ok := err.(*net.OpError); !ok {

			}
		}
	}
}

func (c *SSH) ProxyHttpTransport() http.RoundTripper {
	return &http.Transport{
		Dial:                c.ssh.Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

func (c *SSH) ProxyHttpClient() *http.Client {
	client := http.Client{
		Transport: c.ProxyHttpTransport(),
	}
	return &client
}

func (c *SSH) ExecCmd(cmd string) (string, string, error) {
	session, err := c.ssh.NewSession()
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

func (c *SSH) Client() *ssh.Client {
	return c.ssh
}

func (c *SSH) ExecCmdPipe(cmd string, output chan string) error {
	session, err := c.ssh.NewSession()
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
	_ = session.Start(cmd)
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil || err == io.EOF {
			break
		}
		output <- line
	}
	_ = session.Wait()
	return nil
}

func (c *SSH) Close() {
	_ = c.ssh.Close()
}
