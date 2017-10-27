package main

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/sutils/fsck"
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
	URI    string
	Reader *io.PipeReader
	Writer *io.PipeWriter
}

func (s *SshNetConn) Read(p []byte) (n int, err error) {
	n, err = s.Reader.Read(p)
	return
}
func (s *SshNetConn) Write(p []byte) (n int, err error) {
	n, err = s.Writer.Write(p)
	return
}
func (s *SshNetConn) Close() (err error) {
	if s.Reader != nil {
		err = s.Reader.Close()
	}
	if s.Writer != nil {
		err = s.Writer.Close()
	}
	return
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

type SshFSckConn struct {
	URI    string
	Sid    uint16
	Reader *io.PipeReader
	Writer *io.PipeWriter
}

func (s *SshFSckConn) Read(p []byte) (n int, err error) {
	n, err = s.Reader.Read(p)
	return
}
func (s *SshFSckConn) Write(p []byte) (n int, err error) {
	n, err = s.Writer.Write(p)
	return
}

func (s *SshFSckConn) Close() (err error) {
	if s.Reader != nil {
		err = s.Reader.Close()
	}
	if s.Writer != nil {
		err = s.Writer.Close()
	}
	return
}
func (s *SshFSckConn) String() string {
	return s.URI
}
func (s *SshFSckConn) SetSid(sid uint16) {
	s.Sid = sid
}
func (s *SshFSckConn) GetSid() uint16 {
	return s.Sid
}

func SshPipe(uri string) (fsck.Conn, net.Conn) {
	fsckCon := &SshFSckConn{
		URI: uri,
	}
	netCon := &SshNetConn{
		URI: uri,
	}
	fsckCon.Reader, netCon.Writer = io.Pipe()
	netCon.Reader, fsckCon.Writer = io.Pipe()
	return fsckCon, netCon
}

type SshHost struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	Username string `json:"username"`
	Password string `json:"password"`
	Pty      string `json:"pty"`
}

func ParseSshHost(name, uri string) (host *SshHost, err error) {
	parts := strings.SplitN(uri, "@", 2)
	if len(parts) < 2 {
		err = fmt.Errorf("parse uri(%v) fail", uri)
		return
	}
	user := strings.Split(parts[0], ":")
	host = &SshHost{
		Name:     name,
		URI:      parts[1],
		Username: user[0],
	}
	if len(user) > 1 {
		host.Password = user[1]
	}
	return
}

type outWriter struct {
	Out io.Writer
}

func (o *outWriter) Write(p []byte) (n int, err error) {
	if o.Out != nil {
		n, err = o.Out.Write(p)
	} else {
		n = len(p)
	}
	return
}

type SshSession struct {
	Running bool
	*SshHost
	C       *fsck.Client
	out     outWriter
	fsckCon fsck.Conn
	netCon  net.Conn
	client  *ssh.Client
	session *ssh.Session
	stdin   io.Writer
}

func NewSshSession(c *fsck.Client, host *SshHost) *SshSession {
	return &SshSession{
		SshHost: host,
		C:       c,
	}
}

func (s *SshSession) SetOut(out io.Writer) {
	s.out.Out = out
}

func (s *SshSession) GetOut() io.Writer {
	return s.out.Out
}

func (s *SshSession) Start() (err error) {
	s.fsckCon, s.netCon = SshPipe(s.URI)
	go s.C.Proc(s.URI, s.fsckCon)
	return s.StartSession(s.netCon)
}

func (s *SshSession) StartSession(con net.Conn) (err error) {
	fmt.Printf("%v start connect to %v\n", s.Name, s.URI)
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
	s.session.Stdout = &s.out
	s.session.Stderr = &s.out
	s.stdin, _ = s.session.StdinPipe()
	//// Request pseudo terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:  0, // Disable echoing
		ssh.IGNCR: 1, // Ignore CR on input.
	}
	pty := s.Pty
	if len(pty) < 1 {
		pty = "vt100"
	}
	err = s.session.RequestPty(s.Pty, 80, 40, modes)
	if err != nil {
		return
	}
	// Start remote shell
	err = s.session.Shell()
	if err != nil {
		return
	}
	fmt.Printf("%v handshake success\n", s.Name)
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
	if !s.Running {
		err = s.Start()
		if err != nil {
			return
		}
	}
	n, err = s.stdin.Write(p)
	return
}

func (s *SshSession) Close() (err error) {
	if s.session != nil {
		err = s.session.Close()
	}
	if s.client != nil {
		err = s.client.Close()
	}
	if s.fsckCon != nil {
		s.fsckCon.Close()
	}
	if s.netCon != nil {
		s.netCon.Close()
	}
	return
}
