package main

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sutils/fsck"
	"github.com/sutils/readkey"
	"golang.org/x/crypto/ssh"
)

type NetAddr struct {
	Net string
	URI string
}

func NewNetAddr(net, uri string) *NetAddr {
	return &NetAddr{Net: net, URI: uri}
}

func (n *NetAddr) Network() string {
	return n.Net
}
func (n *NetAddr) String() string {
	return n.URI
}

type SshNetConn struct {
	URI string
	*fsck.Session
}

func NewSshNetConn(uri string, s *fsck.Session) *SshNetConn {
	return &SshNetConn{
		URI:     uri,
		Session: s,
	}
}

func (s *SshNetConn) LocalAddr() net.Addr {
	return NewNetAddr("tcp", "local")
}

func (s *SshNetConn) RemoteAddr() net.Addr {
	return NewNetAddr("tcp", s.URI)
}
func (s *SshNetConn) SetDeadline(t time.Time) error {
	return nil
}
func (s *SshNetConn) SetReadDeadline(t time.Time) error {
	return nil
}
func (s *SshNetConn) SetWriteDeadline(t time.Time) error {
	return nil
}

// type SshFSckConn struct {
// 	URI    string
// 	Reader *io.PipeReader
// 	Writer *io.PipeWriter
// }

// func (s *SshFSckConn) Read(p []byte) (n int, err error) {
// 	n, err = s.Reader.Read(p)
// 	return
// }
// func (s *SshFSckConn) Write(p []byte) (n int, err error) {
// 	n, err = s.Writer.Write(p)
// 	return
// }

// func (s *SshFSckConn) Close() (err error) {
// 	if s.Reader != nil {
// 		err = s.Reader.Close()
// 	}
// 	if s.Writer != nil {
// 		err = s.Writer.Close()
// 	}
// 	return
// }
// func (s *SshFSckConn) String() string {
// 	return s.URI
// }

// func SshPipe(uri string) (*SshFSckConn, net.Conn) {
// 	fsckCon := &SshFSckConn{
// 		URI: uri,
// 	}
// 	netCon := &SshNetConn{
// 		URI: uri,
// 	}
// 	fsckCon.Reader, netCon.Writer = io.Pipe()
// 	netCon.Reader, fsckCon.Writer = io.Pipe()
// 	return fsckCon, netCon
// }

type SshHost struct {
	Name     string   `json:"name"`
	URI      string   `json:"uri"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	Channel  string   `json:"channel"`
	Pty      string   `json:"pty"`
	Env      []string `json:"env"`
}

func ParseSshHost(name, uri string, env map[string]interface{}) (host *SshHost, err error) {
	if !regexp.MustCompile("^.*://.*$").MatchString(uri) {
		uri = "master://" + uri
	}
	ruri, err := url.Parse(uri)
	if err != nil {
		return
	}
	suri := ruri.Host
	if !strings.Contains(suri, ":") {
		suri = suri + ":22"
	}
	channel := ruri.Scheme
	if len(channel) < 1 {
		channel = "master"
	}
	host = &SshHost{
		Name:    name,
		URI:     suri,
		Channel: channel,
	}
	if ruri.User != nil {
		host.Username = ruri.User.Username()
		host.Password, _ = ruri.User.Password()
	}
	pty := ruri.Query().Get("pty")
	if len(pty) > 0 {
		host.Pty = pty
	}
	for key, val := range env {
		host.Env = append(host.Env, fmt.Sprintf("%v=%v", key, val))
	}
	return
}

type SshSession struct {
	Running bool
	*SshHost
	*MultiWriter
	Idx     int
	C       *fsck.Slaver
	out     *OutWriter
	conn    *SshNetConn
	client  *ssh.Client
	session *ssh.Session
	stdin   io.Writer
	Prefix  io.Reader
}

func NewSshSession(c *fsck.Slaver, host *SshHost) *SshSession {
	ss := &SshSession{
		SshHost:     host,
		C:           c,
		out:         NewOutWriter(),
		MultiWriter: NewMultiWriter(),
	}
	ss.MultiWriter.Add(ss.out)
	return ss
}

func (s *SshSession) String() string {
	return s.Name
}

func (s *SshSession) EnableCallback(prefix []byte, back chan []byte) {
	s.out.EnableCallback(prefix, back)
}

func (s *SshSession) DisableCallback() {
	s.out.DisableCallback()
}

func (s *SshSession) Start() (err error) {
	session, err := s.C.DialSession(s.Channel, s.URI)
	if err == nil {
		s.conn = NewSshNetConn(s.URI, session)
		err = s.StartSession(s.conn)
	}
	return
}

func (s *SshSession) StartSession(con net.Conn) (err error) {
	fmt.Printf("%v start connect to %v\n", s.Name, s.URI)
	// create session
	config := &ssh.ClientConfig{
		User: s.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	c, chans, reqs, err := ssh.NewClientConn(con, s.URI, config)
	if err != nil {
		return
	}
	s.client = ssh.NewClient(c, chans, reqs)
	s.session, err = s.client.NewSession()
	if err != nil {
		return
	}
	s.session.Stdout = s.MultiWriter
	s.session.Stderr = s.MultiWriter
	s.stdin, _ = s.session.StdinPipe()
	// Request pseudo terminal
	modes := ssh.TerminalModes{
	// ssh.ECHO:          0,     // Disable echoing
	// ssh.IGNCR:         1,     // Ignore CR on input.
	// ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
	// ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	pty := s.Pty
	if len(pty) < 1 {
		pty = "vt100"
	}
	w, h := readkey.GetSize()
	err = s.session.RequestPty(pty, h, w, modes)
	if err != nil {
		return
	}
	// Start remote shell
	err = s.session.Shell()
	if err != nil {
		return
	}
	fmt.Printf("%v handshake success\n", s.Name)
	s.MultiWriter.Disable = true
	for _, env := range s.Env {
		fmt.Fprintf(s.stdin, "%v\n", env)
	}
	if s.Prefix != nil {
		_, err = io.Copy(s.stdin, s.Prefix)
	}
	time.Sleep(500 * time.Millisecond)
	s.MultiWriter.Disable = false
	s.Running = true
	return
}

func (s *SshSession) Wait() (err error) {
	if s.session == nil {
		err = fmt.Errorf("not started")
		return
	}
	err = s.session.Wait()
	s.Running = false
	return
}

func (s *SshSession) Write(p []byte) (n int, err error) {
	for i := 0; i < 3; i++ {
		if !s.Running {
			err = s.Start()
			if err != nil {
				return
			}
		}
		n, err = s.stdin.Write(p)
		if err == io.EOF {
			s.Running = false
			fmt.Printf("\nsession is closed, will retry to connection\n")
			continue
		}
		break
	}
	return
}

func (s *SshSession) Close() (err error) {
	if s.session != nil {
		err = s.session.Close()
	}
	if s.client != nil {
		err = s.client.Close()
	}
	if s.conn != nil {
		s.conn.Close()
	}
	return
}
