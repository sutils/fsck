package fsck

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Centny/gwf/netw"

	"github.com/Centny/gwf/log"
)

var ShowLog int

// func log_d(format string, args ...interface{}) {
// 	if ShowLog > 1 {
// 		log.D_(1, format, args...)
// 	}
// }

const (
	SS_NEW    = 20
	SS_NORMAL = 0
	SS_CLOSED = 10
)

var ErrSessionNotFound = fmt.Errorf("-session:not found")
var ErrSessionClosed = fmt.Errorf("-session:closed")

var OK = "ok"

type ErrOK struct {
	Data string
}

func (e *ErrOK) Error() string {
	return OK
}

func IsErrOK(err error) bool {
	_, ok := err.(*ErrOK)
	return ok
}

type Session interface {
	net.Conn
	ID() uint16
	RawWrite(p []byte) (n int, err error)
	OnlyClose() (err error)
}

type SessionDialer interface {
	Dial(sid uint16, uri string, out io.Writer) (session Session, err error)
	Bind(sid uint16, out io.Writer) (session Session, err error)
}

type SidSession struct {
	reader   io.ReadCloser
	Raw      io.WriteCloser
	Out      io.Writer
	SID      uint16
	Timeout  time.Duration
	MaxDelay time.Duration
	closed   int32
	OnClose  func(session Session)
}

func NewSidSession(sid uint16, out io.Writer, raw io.WriteCloser) *SidSession {
	var reader io.ReadCloser
	var writer io.WriteCloser
	if raw == nil {
		reader, writer = io.Pipe()
	} else {
		writer = raw
	}
	return &SidSession{
		SID:      sid,
		Out:      out,
		reader:   reader,
		Raw:      writer,
		Timeout:  60 * time.Second,
		MaxDelay: 8 * time.Second,
		OnClose:  func(session Session) {},
	}
}

func (s *SidSession) ID() uint16 {
	return s.SID
}

func (s *SidSession) Write(p []byte) (n int, err error) {
	// log.D("Session write data:%v", string(p))
	buf := append(make([]byte, 3), p...)
	binary.BigEndian.PutUint16(buf[1:], s.SID)
	var waited time.Duration
	var tempDelay time.Duration
	for {
		_, err = s.Out.Write(buf)
		if err == nil || IsErrOK(err) {
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
		log.D("SidSession(%v) send %v data fail with %v, will retry after %v", s.SID, len(buf), err, tempDelay)
		time.Sleep(tempDelay)
		waited += tempDelay
		if waited > s.Timeout {
			log.W("SidSession wait channel on sid(%v) fail with timeout", s.SID)
			err = io.EOF
			//err = fmt.Errorf("timeout")
			s.Close()
			break
		}
	}
	n = len(p)
	return
}

func (s *SidSession) Read(p []byte) (n int, err error) {
	if s.reader == nil {
		panic("raw write mode is not having reader")
	}
	n, err = s.reader.Read(p)
	return
}

func (s *SidSession) RawWrite(p []byte) (n int, err error) {
	n, err = s.Raw.Write(p)
	return
}

func (s *SidSession) Close() (err error) {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		err = s.Raw.Close()
		s.OnClose(s)
	}
	return
}
func (s *SidSession) OnlyClose() (err error) {
	if atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		err = s.Raw.Close()
	}
	return
}

func (s *SidSession) LocalAddr() net.Addr {
	return s
}
func (s *SidSession) RemoteAddr() net.Addr {
	return s
}
func (s *SidSession) SetDeadline(t time.Time) error {
	return nil
}
func (s *SidSession) SetReadDeadline(t time.Time) error {
	return nil
}
func (s *SidSession) SetWriteDeadline(t time.Time) error {
	return nil
}
func (s *SidSession) Network() string {
	return "session"
}
func (s *SidSession) String() string {
	return fmt.Sprintf("session(%v)", s.SID)
}

type SessionPool struct {
	ss              map[uint16]Session
	lck             sync.RWMutex
	wg              sync.WaitGroup
	Dialers         []Dialer
	OnSessionClosed func(session Session)
}

func NewSessionPool() *SessionPool {
	return &SessionPool{
		ss:              map[uint16]Session{},
		lck:             sync.RWMutex{},
		wg:              sync.WaitGroup{},
		OnSessionClosed: func(session Session) {},
	}
}

func (s *SessionPool) RegisterDefaulDialer() (err error) {
	for _, dialer := range []Dialer{NewCmdDialer(), NewEchoDialer(), NewWebDialer(), NewTCPDialer()} {
		err = s.AddDialer(dialer)
		if err != nil {
			return
		}
	}
	return
}

func (s *SessionPool) AddDialer(dialer Dialer) error {
	err := dialer.Bootstrap()
	if err != nil {
		log.E("SessionPool bootstrap dialer fail with %v", err)
		return err
	}
	s.Dialers = append(s.Dialers, dialer)
	return nil
}

func (s *SessionPool) Dial(sid uint16, uri string, out io.Writer) (session Session, err error) {
	var raw io.ReadWriteCloser
	err = fmt.Errorf("not matched dialer for %v", uri)
	for _, dialer := range s.Dialers {
		if dialer.Matched(uri) {
			log.D("SessionPool will use %v to dial by %v", dialer, uri)
			raw, err = dialer.Dial(sid, uri)
			break
		}
	}
	if err == nil {
		session = s.Bind(sid, out, raw)
		s.wg.Add(1)
		go s.copy(session, raw)
	}
	return
}

func (s *SessionPool) copy(session Session, raw io.ReadWriteCloser) {
	var buf []byte
	if netw.MOD_MAX_SIZE == 4 {
		buf = make([]byte, 1024*1024)
	} else {
		buf = make([]byte, 60000)
	}
	var readed int
	var err error
	for {
		readed, err = raw.Read(buf)
		if err == nil && readed > 0 {
			_, err = session.Write(buf[0:readed])
		}
		if err != nil && !IsErrOK(err) {
			break
		}
	}
	log.D("SessionPool the session(%v) reader is close by %v", session.ID(), err)
	session.Close()
	raw.Close()
	s.Remove(session.ID())
	s.wg.Done()
}

func (s *SessionPool) Start(sid uint16, out io.Writer) (session Session) {
	return s.Bind(sid, out, nil)
}

func (s *SessionPool) Bind(sid uint16, out io.Writer, raw io.WriteCloser) (session Session) {
	s.lck.Lock()
	defer s.lck.Unlock()
	sids := NewSidSession(sid, out, raw)
	sids.OnClose = func(base Session) {
		s.lck.Lock()
		delete(s.ss, sid)
		s.lck.Unlock()
		s.OnSessionClosed(base)
	}
	s.ss[sid] = sids
	session = sids
	return
}

func (s *SessionPool) Find(sid uint16) (session Session) {
	s.lck.RLock()
	defer s.lck.RUnlock()
	session, _ = s.ss[sid]
	return
}

func (s *SessionPool) Remove(sid uint16) (session Session) {
	s.lck.Lock()
	defer s.lck.Unlock()
	session, _ = s.ss[sid]
	if session != nil {
		session.OnlyClose()
		delete(s.ss, sid)
	}
	return
}

func (s *SessionPool) Write(p []byte) (n int, err error) {
	if len(p) < 3 {
		err = fmt.Errorf("frame must be greater 3 bytes")
		log.E("SessionPool receive data fail with %v", err)
		return
	}
	sid := binary.BigEndian.Uint16(p[1:])
	session := s.Find(sid)
	if session == nil {
		log.D("SesssionPool find session fail by sid(%v)", sid)
		err = ErrSessionNotFound
		return
	}
	if ShowLog > 1 {
		log.D("SessionPool send %v data to session(%v)", len(p)-3, sid)
	}
	n, err = session.RawWrite(p[3:])
	if err != nil {
		session.Close()
		err = ErrSessionClosed
	}
	return
}

func (s *SessionPool) Close() error {
	s.lck.Lock()
	for _, ss := range s.ss {
		ss.OnlyClose()
	}
	s.lck.Unlock()
	s.wg.Wait()
	return nil
}
