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

const (
	SS_NEW    = 20
	SS_NORMAL = 0
	SS_CLOSED = 10
)

var ErrSessionNotFound = fmt.Errorf("-session:not found")
var ErrSessionClosed = fmt.Errorf("-session:closed")

var OK = "ok"

type Session struct {
	Raw io.ReadWriteCloser
	SID uint16
}

func (s *Session) runRead(w io.Writer) {
	buf := make([]byte, 40960)
	buf[0] = 0
	binary.BigEndian.PutUint16(buf[1:], s.SID)
	for {
		readed, err := s.Raw.Read(buf[3:])
		if err != nil {
			break
		}
		var waited int64
		for {
			_, err = w.Write(buf[:readed+3])
			if err == nil {
				break
			}
			if err == ErrSessionClosed {
				log.D("remote session(%v) is closed")
				break
			}
			if err == ErrSessionNotFound {
				log.D("remote session(%v) is not found")
				break
			}
			log.D("Sessiion(%v) send %v data fail with %v, will retry after %v", s.SID, readed+3, err, "500ms")
			time.Sleep(500 * time.Millisecond)
			waited += 100
			if waited > 60000 {
				log.W("Server wait channel on sid(%v) fail with timeout", s.SID)
				break
			}
		}

	}
	s.Raw.Close()
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

func (s *SessionPool) Dail(uri string, w io.Writer) (session *Session, err error) {
	raw, err := net.Dial("tcp", uri)
	if err != nil {
		return
	}
	s.lck.Lock()
	defer s.lck.Unlock()
	s.sidc++
	sid := s.sidc
	session = s.start(raw, sid, w)
	return
}

func (s *SessionPool) Start(raw io.ReadWriteCloser, sid uint16, w io.Writer) (session *Session) {
	s.lck.Lock()
	defer s.lck.Unlock()
	return s.start(raw, sid, w)
}

func (s *SessionPool) start(raw io.ReadWriteCloser, sid uint16, w io.Writer) (session *Session) {
	session = &Session{Raw: raw, SID: sid}
	s.ss[sid] = session
	go session.runRead(w)
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
		session.Raw.Close()
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
		err = ErrSessionNotFound
		return
	}
	n, err = session.Raw.Write(p[3:])
	if err != nil {
		s.Remove(sid)
		err = ErrSessionClosed
	}
	return
}
