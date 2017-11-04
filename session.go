package fsck

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Centny/gwf/log"
)

var ShowLog int

const (
	SS_NEW    = 20
	SS_NORMAL = 0
	SS_CLOSED = 10
)

var ErrSessionNotFound = fmt.Errorf("-session:not found")
var ErrSessionClosed = fmt.Errorf("-session:closed")

var OK = "ok"

type Session struct {
	io.ReadWriteCloser
	reader   io.ReadCloser
	Raw      io.WriteCloser
	Out      io.Writer
	SID      uint16
	Timeout  time.Duration
	MaxDelay time.Duration
}

func NewSession(sid uint16, out io.Writer, raw io.WriteCloser) *Session {
	var reader io.ReadCloser
	var writer io.WriteCloser
	if raw == nil {
		reader, writer = io.Pipe()
	} else {
		writer = raw
	}
	return &Session{
		SID:      sid,
		Out:      out,
		reader:   reader,
		Raw:      writer,
		Timeout:  60 * time.Second,
		MaxDelay: 8 * time.Second,
	}
}

func (s *Session) Write(p []byte) (n int, err error) {
	// log.D("Session write data:%v", string(p))
	buf := append(make([]byte, 3), p...)
	binary.BigEndian.PutUint16(buf[1:], s.SID)
	var waited time.Duration
	var tempDelay time.Duration
	for {
		_, err = s.Out.Write(buf)
		if err == nil {
			break
		}
		if err == ErrSessionClosed {
			log.D("remote session(%v) is closed", s.SID)
			s.Close()
			err = io.EOF
			// err = fmt.Errorf("remote session(%v) is closed", s.SID)
			break
		}
		if err == ErrSessionNotFound {
			log.D("remote session(%v) is not found", s.SID)
			s.Close()
			err = io.EOF
			// err = fmt.Errorf("remote session(%v) is not found", s.SID)
			break
		}
		if tempDelay == 0 {
			tempDelay = 100 * time.Millisecond
		} else {
			tempDelay *= 2
		}
		if tempDelay > s.MaxDelay {
			tempDelay = s.MaxDelay
		}
		log.D("Sessiion(%v) send %v data fail with %v, will retry after %v", s.SID, len(buf), err, tempDelay)
		time.Sleep(tempDelay)
		waited += tempDelay
		if waited > s.Timeout {
			log.W("Server wait channel on sid(%v) fail with timeout", s.SID)
			err = io.EOF
			//err = fmt.Errorf("timeout")
			s.Close()
			break
		}
	}
	n = len(p)
	return
}

func (s *Session) Read(p []byte) (n int, err error) {
	if s.reader == nil {
		panic("raw write mode is not having reader")
	}
	n, err = s.reader.Read(p)
	return
}

func (s *Session) writeToReader(p []byte) (n int, err error) {
	n, err = s.Raw.Write(p)
	return
}

func (s *Session) Close() (err error) {
	err = s.Raw.Close()
	log.D("Session(%v) is closed", s.SID)
	return
}

type SessionPool struct {
	ss   map[uint16]*Session
	sidc uint16
	lck  sync.RWMutex
}

func NewSessionPool() *SessionPool {
	return &SessionPool{
		ss:  map[uint16]*Session{},
		lck: sync.RWMutex{},
	}
}

func (s *SessionPool) Dail(uri string, out io.Writer) (session *Session, err error) {
	raw, err := net.Dial("tcp", uri)
	if err != nil {
		return
	}
	s.lck.Lock()
	defer s.lck.Unlock()
	s.sidc++
	sid := s.sidc
	session = s.start(sid, out, raw)
	go func() {
		io.Copy(session, raw)
		session.Close()
		raw.Close()
		s.Remove(sid)
	}()
	return
}

func (s *SessionPool) Start(sid uint16, out io.Writer) (session *Session) {
	s.lck.Lock()
	defer s.lck.Unlock()
	return s.start(sid, out, nil)
}

func (s *SessionPool) start(sid uint16, out io.Writer, raw io.WriteCloser) (session *Session) {
	session = NewSession(sid, out, raw)
	s.ss[sid] = session
	return
}

func (s *SessionPool) Find(sid uint16) (session *Session) {
	s.lck.RLock()
	defer s.lck.RUnlock()
	session, _ = s.ss[sid]
	return
}

func (s *SessionPool) Remove(sid uint16) (session *Session) {
	s.lck.Lock()
	defer s.lck.Unlock()
	session, _ = s.ss[sid]
	if session != nil {
		session.Close()
		delete(s.ss, sid)
	}
	return
}

func (s *SessionPool) Write(p []byte) (n int, err error) {
	if len(p) < 3 {
		err = fmt.Errorf("frame must be greater 3 bytes")
		return
	}
	sid := binary.BigEndian.Uint16(p[1:])
	session := s.Find(sid)
	if session == nil {
		log.D("SesssionPool(%p) find session fail by sid(%v)", s, sid)
		err = ErrSessionNotFound
		return
	}
	if ShowLog > 1 {
		log.D("SessionPool send %v data to session(%v)", len(p)-3, sid)
	}
	n, err = session.writeToReader(p[3:])
	if err != nil {
		s.Remove(sid)
		err = ErrSessionClosed
	}
	return
}
