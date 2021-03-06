package fsck

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/net/websocket"

	"github.com/Centny/gwf/log"
	"github.com/Centny/gwf/routing"
	"github.com/Centny/gwf/util"
)

type Mapping struct {
	Name    string   `json:"name"`
	Channel string   `json:"channel"`
	Local   *url.URL `json:"local"`
	Remote  *url.URL `json:"remote"`
}

func NewMapping(name, uri string) (mapping *Mapping, err error) {
	parts := regexp.MustCompile("[<>]").Split(uri, 3)
	if len(parts) != 3 {
		err = fmt.Errorf("invalid uri:%v", uri)
		return
	}
	mapping = &Mapping{}
	mapping.Name = name
	mapping.Channel = parts[1]
	mapping.Local, err = url.Parse(parts[0])
	if err == nil {
		mapping.Remote, err = url.Parse(parts[2])
	}
	return
}

func (m *Mapping) LocalValidF(format string, args ...interface{}) error {
	return util.ValidAttrF(format, m.Local.Query().Get, true, args...)
}

func (m *Mapping) RemoteValidF(format string, args ...interface{}) error {
	return util.ValidAttrF(format, m.Remote.Query().Get, true, args...)
}

func (m *Mapping) String() string {
	return fmt.Sprintf("%v<%v>%v", m.Local, m.Channel, m.Remote)
}

type MappingSorter []*Mapping

func (f MappingSorter) Len() int {
	return len(f)
}

func (f MappingSorter) Less(i, j int) bool {
	if f[i].Name == f[j].Name {
		return f[i].Local.String() < f[j].Local.String()
	}
	return f[i].Name < f[j].Name
}

func (f MappingSorter) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

type ChannelInfo struct {
	Name   string
	Online bool
	MS     []*Mapping
	Remote string
}

type ForwardDialerF func(channel, uri string, raw io.WriteCloser) (session Session, err error)

type Forward struct {
	ls         map[string]*ForwardListener
	ms         map[string]*Mapping
	cs         map[string]Session
	stop       map[string]chan int
	webMapping map[string]*Mapping
	wsMapping  map[string]*Mapping
	lck        sync.RWMutex
	WebPrefix  string
	WebSuffix  string
	WebAuth    string
	Dialer     ForwardDialerF
}

func NewForward(dialer ForwardDialerF) *Forward {
	return &Forward{
		ls:         map[string]*ForwardListener{},
		ms:         map[string]*Mapping{},
		cs:         map[string]Session{},
		stop:       map[string]chan int{},
		webMapping: map[string]*Mapping{},
		wsMapping:  map[string]*Mapping{},
		lck:        sync.RWMutex{},
		Dialer:     dialer,
	}
}

func (f *Forward) AddUriForward(name, uri string) (mapping *Mapping, err error) {
	mapping, err = NewMapping(name, uri)
	if err == nil {
		err = f.AddForward(mapping)
	}
	return
}

func (f *Forward) AddForward(m *Mapping) (err error) {
	f.lck.Lock()
	defer f.lck.Unlock()
	if _, ok := f.ms[m.Name]; ok {
		err = fmt.Errorf("the forward is exsits by name(%v)", m.Name)
		return
	}
	switch m.Local.Scheme {
	case "tcp":
		if _, ok := f.ls[m.Local.Host]; ok {
			err = fmt.Errorf("the forward is exsits by local(%v)", m.Local)
			return
		}
		var l *ForwardListener
		l, err = NewForwardListener(m)
		if err != nil {
			log.W("Forward add tcp forward by %v fail with %v", m, err)
			return
		}
		if len(m.Local.Host) < 1 {
			m.Local.Host = l.Addr().String()
		}
		f.ms[m.Name] = m
		f.ls[m.Local.Host] = l
		f.stop[m.Name] = make(chan int)
		go f.accept(m, l, m.Channel, m.Remote.String())
		log.D("Forward add tcp forward by %v success", m)
	case "web":
		if _, ok := f.webMapping[m.Local.Host]; ok {
			err = fmt.Errorf("web host key(%v) is exists", m.Local.Host)
			log.W("Forward add web forward by %v fail with key exists", m)
		} else {
			f.ms[m.Name] = m
			f.webMapping[m.Local.Host] = m
			log.D("Forward add web forward by %v success", m)
		}
	case "ws":
		if _, ok := f.wsMapping[m.Local.Host]; ok {
			err = fmt.Errorf("ws host key(%v) is exists", m.Local.Host)
			log.W("Forward add ws forward by %v fail with key exists", m)
		} else {
			f.ms[m.Name] = m
			f.wsMapping[m.Local.Host] = m
			log.D("Forward add ws forward by %v success", m)
		}
	default:
		err = fmt.Errorf("scheme %v is not suppored", m.Local.Scheme)
	}
	return
}

func (f *Forward) RemoveForward(local string) (err error) {
	rurl, err := url.Parse(local)
	if err != nil {
		return
	}
	f.lck.Lock()
	defer f.lck.Unlock()
	switch rurl.Scheme {
	case "tcp":
		listener := f.ls[rurl.Host]
		if listener != nil {
			listener.Close()
			delete(f.ls, rurl.Host)
			delete(f.ms, listener.Name)
			log.D("Forward removing forward by %v success", local)
		} else {
			err = fmt.Errorf("forward is not exitst")
			log.D("Forward removing forward by %v fail with not exists", local)
		}
	case "web":
		forward := f.webMapping[rurl.Host]
		if forward != nil {
			delete(f.webMapping, rurl.Host)
			delete(f.ms, forward.Name)
			log.D("Forward removing web forward by %v success", local)
		} else {
			err = fmt.Errorf("web forward is not exist by %v", local)
			log.D("Forward removing web forward by %v fail with not exists", local)
		}
	case "ws":
		fallthrough
	case "wss":
		forward := f.wsMapping[rurl.Host]
		if forward != nil {
			delete(f.wsMapping, rurl.Host)
			delete(f.ms, forward.Name)
			log.D("Forward removing ws forward by %v success", local)
		} else {
			err = fmt.Errorf("ws forward is not exist by %v", local)
			log.D("Forward removing ws forward by %v fail with not exists", local)
		}
	default:
		err = fmt.Errorf("scheme %v is not suppored", rurl.Scheme)
	}
	return
}

func (f *Forward) AllForwards() (mapping map[string][]*Mapping) {
	f.lck.RLock()
	defer f.lck.RUnlock()
	mapping = map[string][]*Mapping{}
	for _, l := range f.ls {
		mapping[l.Channel] = append(mapping[l.Channel], l.Mapping)
	}
	for _, m := range f.webMapping {
		mapping[m.Channel] = append(mapping[m.Channel], m)
	}
	for _, m := range f.wsMapping {
		mapping[m.Channel] = append(mapping[m.Channel], m)
	}
	return
}

func (f *Forward) accept(m *Mapping, listen net.Listener, channel, uri string) {
	var limit int
	err := m.LocalValidF(`limit,O|I,R:-1`, &limit)
	if err != nil {
		log.W("Forward(%v) forward listener(%v) get the limit valid fail with %v", m.Name, m.Local, err)
	}
	log.D("Forward(%v) run forward listener(%v) with limit:%v", m.Name, m, limit)
	for {
		conn, err := listen.Accept()
		if err != nil {
			log.D("Forwad(%v) accept fail with %v", m.Name, err)
			break
		}
		session, err := f.Dialer(channel, uri, conn)
		if err != nil {
			log.E("Forward(%v) dial new session by channel(%v),uri(%v) fail with %v", m.Name, channel, uri, err)
			conn.Close()
			continue
		}
		log.D("Forward(%v) bind session(%v) on %v success", m.Name, session.ID(), conn.RemoteAddr())
		f.lck.Lock()
		f.cs[fmt.Sprintf("%v-%v", m.Name, session.ID())] = session
		f.lck.Unlock()
		go f.copy(m, conn, session)
		if limit > 0 {
			limit--
			if limit < 1 {
				listen.Close()
			}
		}
	}
	listen.Close()
	f.lck.Lock()
	delete(f.ms, m.Name)
	delete(f.ls, m.Local.Host)
	stop, ok := f.stop[m.Name]
	delete(f.stop, m.Name)
	f.lck.Unlock()
	log.D("Forward(%v) is stopped", m.Name)
	if ok {
		stop <- 1
	}

}
func (f *Forward) copy(m *Mapping, conn net.Conn, session Session) {
	// go func() {
	// 	io.Copy(conn, session)
	// 	conn.Close()
	// 	session.Close()
	// }()
	_, err := io.Copy(session, conn)
	conn.Close()
	session.Close()
	f.lck.Lock()
	delete(f.cs, fmt.Sprintf("%v-%v", m.Name, session.ID()))
	f.lck.Unlock()
	log.D("Forwad(%v) connect from %v is closed by %v", m.Name, conn.RemoteAddr(), err)
}

func (f *Forward) Stop(name string, connected bool) (err error) {
	f.lck.RLock()
	if connected {
		for key, session := range f.cs {
			if strings.HasPrefix(key, name+"-") {
				session.Close()
			}
		}
	}
	var listener *ForwardListener
	m := f.ms[name]
	stop := f.stop[name]
	if m != nil {
		listener = f.ls[m.Local.Host]
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

func (f *Forward) ProcWebSubsH(hs *routing.HTTPSession) routing.HResult {
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(hs.R.URL.Path, f.WebPrefix), "/"), "/")
	return f.ProcName(parts[0], hs)
}

func (f *Forward) HostForwardF(hs *routing.HTTPSession) routing.HResult {
	host := hs.R.Host
	if len(f.WebSuffix) > 0 && strings.HasSuffix(host, f.WebSuffix) {
		name := strings.Trim(strings.TrimSuffix(host, f.WebSuffix), ". ")
		if len(name) > 0 {
			return f.ProcName(name, hs)
		}
	}
	return routing.HRES_CONTINUE
}

func (f *Forward) ProcName(name string, hs *routing.HTTPSession) routing.HResult {
	connection := hs.R.Header.Get("Connection")
	log.D("Forward proc web by name(%v),Connection(%v)", name, connection)
	var mapping *Mapping
	f.lck.RLock()
	if connection == "Upgrade" {
		mapping = f.wsMapping[name]
	} else {
		mapping = f.webMapping[name]
	}
	f.lck.RUnlock()
	if mapping == nil {
		hs.W.WriteHeader(404)
		log.W("Forward proc web by name(%v),Connection(%v) fail with not found", name, connection)
		return hs.Printf("alias not exist by name:%v", name)
	}
	if connection == "Upgrade" {
		websocket.Handler(func(conn *websocket.Conn) {
			f.runWebsocket(conn, mapping)
		}).ServeHTTP(hs.W, hs.R)
		return routing.HRES_RETURN
	}
	if len(f.WebAuth) > 0 && mapping.Local.Query().Get("auth") != "0" {
		username, password, ok := hs.R.BasicAuth()
		if !(ok && f.WebAuth == fmt.Sprintf("%v:%s", username, password)) {
			hs.W.Header().Set("WWW-Authenticate", "Basic realm=Reverse Server")
			hs.W.WriteHeader(401)
			return hs.Printf("%v", "401 Unauthorized")
		}
	}
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Host = req.Host
			req.URL.Scheme = mapping.Remote.Scheme
		},
		Transport: &http.Transport{
			Dial: func(network, addr string) (raw net.Conn, err error) {
				return f.procDial(network, mapping.Remote.Host, mapping)
			},
			DialTLS: func(network, addr string) (raw net.Conn, err error) {
				return f.procDialTLS(network, mapping.Remote.Host, mapping)
			},
		},
	}
	proxy.ServeHTTP(hs.W, hs.R)
	return routing.HRES_RETURN
}

func (f *Forward) runWebsocket(conn *websocket.Conn, mapping *Mapping) {
	raw, err := f.Dialer(mapping.Channel, mapping.Remote.String(), conn)
	if err != nil {
		conn.Close()
		return
	}
	io.Copy(raw, conn)
	raw.Close()
	conn.Close()
}

func (f *Forward) procDial(network, addr string, mapping *Mapping) (raw net.Conn, err error) {
	raw, err = f.Dialer(mapping.Channel, mapping.Remote.String(), nil)
	return
}

func (f *Forward) procDialTLS(network, addr string, mapping *Mapping) (raw net.Conn, err error) {
	rawCon, err := f.Dialer(mapping.Channel, mapping.Remote.String(), nil)
	if err != nil {
		return
	}
	tlsConn := tls.Client(rawCon, &tls.Config{InsecureSkipVerify: true})
	err = tlsConn.Handshake()
	if err == nil {
		raw = tlsConn
	} else {
		rawCon.Close()
		tlsConn.Close()
	}
	return
}

type ForwardListener struct {
	*Mapping
	net.Listener
}

func NewForwardListener(m *Mapping) (l *ForwardListener, err error) {
	l = &ForwardListener{
		Mapping: m,
	}
	if len(m.Local.Host) > 0 {
		l.Listener, err = net.Listen("tcp", m.Local.Host)
		return
	}
	l.Listener, err = net.Listen("tcp", "127.0.0.1:0")
	return
}

func (f *ForwardListener) String() string {
	return fmt.Sprintf("%v", f.Mapping)
}

// func NewLocalListener(addr string) (l net.Listener, err error) {
// 	if len(addr) > 0 {
// 		l, err = net.Listen("tcp", addr)
// 		return
// 	}
// 	l, err = net.Listen("tcp", "127.0.0.1:0")
// 	// if err != nil {
// 	// 	l, err = net.Listen("tcp6", "[::1]:0")
// 	// }
// 	return
// }
