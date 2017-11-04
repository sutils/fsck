package fsck

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/Centny/gwf/log"
)

type Mapping struct {
	Name   string `json:"name"`
	Local  string `json:"local"`
	Remote string `json:"remote"`
}

type Forward struct {
	ls     map[string]net.Listener
	ms     map[string]*Mapping
	cs     map[string]*Session
	stop   map[string]chan int
	lck    sync.RWMutex
	Client *Slaver
}

func NewForward(client *Slaver) *Forward {
	return &Forward{
		ls:     map[string]net.Listener{},
		ms:     map[string]*Mapping{},
		cs:     map[string]*Session{},
		stop:   map[string]chan int{},
		lck:    sync.RWMutex{},
		Client: client,
	}
}

func (f *Forward) Start(m *Mapping) (listen net.Listener, err error) {
	f.lck.Lock()
	defer f.lck.Unlock()
	if _, ok := f.ms[m.Name]; ok {
		err = fmt.Errorf("the forward is exsits by name(%v)", m.Name)
		return
	}
	if _, ok := f.ls[m.Local]; ok {
		err = fmt.Errorf("the forward is exsits by local(%v)", m.Local)
		return
	}
	var uri = m.Remote
	if !regexp.MustCompile("^.*://.*$").MatchString(uri) {
		uri = "master://" + uri
	}
	ruri, err := url.Parse(uri)
	if err != nil {
		return
	}
	listen, err = NewLocalListener(m.Local)
	if err != nil {
		return
	}
	if len(m.Local) < 1 {
		m.Local = strings.TrimPrefix(listen.Addr().String(), "127.0.0.1")
	}
	f.ms[m.Name] = m
	f.ls[m.Local] = listen
	f.stop[m.Name] = make(chan int)
	go f.accept(m, listen, ruri.Scheme, ruri.Host)
	return
}

func (f *Forward) accept(m *Mapping, listen net.Listener, channel, uri string) {
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.D("Forwad(%v) accept %v fail with %v", m.Name, m.Local, err)
			break
		}
		session, err := f.Client.DialSession(channel, uri)
		if err != nil {
			log.E("Forward(%v) dail new session by channel(%v),uri(%v) fail with %v", m.Name, channel, uri, err)
			conn.Close()
			continue
		}
		log.D("Forward(%v) bind session(%v) on %v success", m.Name, session.SID, conn.RemoteAddr())
		f.lck.Lock()
		f.cs[fmt.Sprintf("%v-%v", m.Name, session.SID)] = session
		f.lck.Unlock()
		go f.copy(m, conn, session)
	}
	listen.Close()
	f.lck.Lock()
	delete(f.ms, m.Name)
	delete(f.ls, m.Local)
	stop, ok := f.stop[m.Name]
	delete(f.stop, m.Name)
	f.lck.Unlock()
	log.D("Forward(%v) is stopped", m.Name)
	if ok {
		stop <- 1
	}

}
func (f *Forward) copy(m *Mapping, conn net.Conn, session *Session) {
	go func() {
		io.Copy(session, conn)
		conn.Close()
		session.Close()
	}()
	_, err := io.Copy(conn, session)
	conn.Close()
	session.Close()
	f.lck.Lock()
	delete(f.cs, fmt.Sprintf("%v-%v", m.Name, session.SID))
	f.lck.Unlock()
	log.D("Forwad(%v) connect from %v is closed by %v", m.Name, conn.RemoteAddr(), err)
}

func (f *Forward) Stop(name string, connected bool) (err error) {
	f.lck.RLock()
	if connected {
		for key, session := range f.cs {
			if strings.HasPrefix(key, name+"-") {
				f.Client.CloseSession(session.SID)
			}
		}
	}
	var listener net.Listener
	m := f.ms[name]
	stop := f.stop[name]
	if m != nil {
		listener = f.ls[m.Local]
	}
	f.lck.RUnlock()
	if listener != nil {
		listener.Close()
		<-stop
	} else {
		err = fmt.Errorf("Forward(%v) is not running", name)
	}
	return
}

func (f *Forward) List() (ms []*Mapping) {
	f.lck.RLock()
	defer f.lck.RUnlock()
	for _, m := range f.ms {
		ms = append(ms, m)
	}
	return
}

func (f *Forward) Close() error {
	ms := f.List()
	for _, m := range ms {
		f.Stop(m.Name, true)
	}
	return nil
}

func NewLocalListener(addr string) (l net.Listener, err error) {
	if len(addr) > 0 {
		l, err = net.Listen("tcp", addr)
		return
	}
	l, err = net.Listen("tcp", "127.0.0.1:0")
	// if err != nil {
	// 	l, err = net.Listen("tcp6", "[::1]:0")
	// }
	return
}
