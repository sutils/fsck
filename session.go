package fsck

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/Centny/gwf/log"
	"github.com/Centny/gwf/util"
)

var ShowLog int

func log_d(format string, args ...interface{}) {
	if ShowLog > 1 {
		log.D_(1, format, args...)
	}
}

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
	ss  map[uint16]*Session
	lck sync.RWMutex
	wg  sync.WaitGroup
}

func NewSessionPool() *SessionPool {
	return &SessionPool{
		ss:  map[uint16]*Session{},
		lck: sync.RWMutex{},
		wg:  sync.WaitGroup{},
	}
}

func (s *SessionPool) Dail(sid uint16, uri string, out io.Writer) (session *Session, err error) {
	var raw io.ReadWriteCloser
	if uri == "echo" {
		raw = NewEchoReadWriteCloser()
	} else {
		raw, err = net.Dial("tcp", uri)
		if err != nil {
			return
		}
	}
	session = s.start(sid, out, raw)
	s.wg.Add(1)
	go s.copy(session, raw)
	return
}

func (s *SessionPool) copy(session *Session, raw io.ReadWriteCloser) {
	buf := make([]byte, 32*1024)
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
	session.Close()
	raw.Close()
	s.Remove(session.SID)
	s.wg.Done()
}

func (s *SessionPool) Start(sid uint16, out io.Writer) (session *Session) {
	return s.start(sid, out, nil)
}

func (s *SessionPool) start(sid uint16, out io.Writer, raw io.WriteCloser) (session *Session) {
	s.lck.Lock()
	defer s.lck.Unlock()
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
	n, err = session.writeToReader(p[3:])
	if err != nil {
		s.Remove(sid)
		err = ErrSessionClosed
	}
	return
}

func (s *SessionPool) Close() error {
	s.lck.Lock()
	for _, ss := range s.ss {
		ss.Close()
	}
	s.lck.Unlock()
	s.wg.Wait()
	return nil
}

type EchoReadWriteCloser struct {
	pipe chan []byte
	lck  sync.RWMutex
}

func NewEchoReadWriteCloser() *EchoReadWriteCloser {
	return &EchoReadWriteCloser{
		pipe: make(chan []byte, 1),
		lck:  sync.RWMutex{},
	}
}

func (e *EchoReadWriteCloser) Write(p []byte) (n int, err error) {
	if e.pipe == nil {
		err = io.EOF
		return
	}
	n = len(p)
	e.pipe <- p[:]
	return
}

func (e *EchoReadWriteCloser) Read(p []byte) (n int, err error) {
	if e.pipe == nil {
		err = io.EOF
		return
	}
	buf := <-e.pipe
	if buf == nil {
		err = io.EOF
		return
	}
	n = copy(p, buf)
	return
}

func (e *EchoReadWriteCloser) Close() (err error) {
	e.lck.Lock()
	if e.pipe != nil {
		e.pipe <- nil
		close(e.pipe)
		e.pipe = nil
	}
	e.lck.Unlock()
	return
}

type EchoPing struct {
	S *Session
}

func NewEchoPing(s *Session) *EchoPing {
	return &EchoPing{S: s}
}

func (e *EchoPing) Ping(data string) (used, slaverCall, slaverBack int64, err error) {
	beg := util.Now()
	_, err = e.S.Write([]byte(data))
	if err == nil {
		err = fmt.Errorf("ping reply err is nil")
		return
	}
	//
	okerr, ok := err.(*ErrOK)
	if !ok {
		return
	}
	ms, err := util.Json2Map(okerr.Data)
	if err != nil {
		err = fmt.Errorf("parse call reply to map fail with %v", err)
		return
	}
	slaverCall = ms.IntValV("used", -1)
	//
	buf := make([]byte, len(data)+64)
	readed, err := e.S.Read(buf)
	if err != nil && readed != len(data)+16 {
		err = fmt.Errorf("ping reply read data fail readed(%v),error(%v)", readed, err)
		return
	}
	buf = buf[readed-16:]
	slaverBeg := int64(binary.BigEndian.Uint64(buf))
	slaverEnd := int64(binary.BigEndian.Uint64(buf[8:]))
	slaverBack = slaverEnd - slaverBeg
	used = util.Now() - beg
	return
}
