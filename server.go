package fsck

import (
	"encoding/binary"
	"fmt"
	"sync"
	"time"

	"github.com/Centny/gwf/log"
	"github.com/Centny/gwf/netw"
	"github.com/Centny/gwf/netw/impl"
	"github.com/Centny/gwf/netw/rc"
	"github.com/Centny/gwf/pool"
	"github.com/Centny/gwf/util"
)

const (
	ChannelCmdS = 100
	ChannelCmdC = 110

	//
	TypeSlaver = "slaver"
	TypeClient = "client"
)

type Server struct {
	*Master
	Local *Slaver
}

func NewServer() *Server {
	var srv = &Server{
		Master: NewMaster(),
		Local:  NewSlaver("local"),
	}
	return srv
}

func (s *Server) Run(addr string, ts map[string]int) error {
	if ts == nil {
		ts = map[string]int{}
	}
	token := util.UUID()
	ts[token] = 1
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := s.Local.StartSlaver(addr, "master", token)
		if err != nil {
			panic(err)
		}
	}()
	err := s.Master.Run(addr, ts)
	if err == nil {
		s.Master.Wait()
	}
	return err
}

func (s *Server) Close() error {
	if s.Master != nil {
		s.Master.Close()
	}
	if s.Local != nil {
		s.Local.Close()
	}
	return nil
}

type Master struct {
	L       *rc.RC_Listener_m
	HbDelay int64
	slck    sync.RWMutex
	slavers map[string]string //slaver name map to connect id
	clients map[string]string //client session map to connect id
	ni2s    map[string]string //mapping <name-sid> to session
	si2n    map[string]string //mapping <session-sid> to name
}

func NewMaster() *Master {
	srv := &Master{
		slck:    sync.RWMutex{},
		slavers: map[string]string{},
		clients: map[string]string{},
		ni2s:    map[string]string{},
		si2n:    map[string]string{},
	}
	return srv
}

func (m *Master) Run(rcaddr string, ts map[string]int) (err error) {
	m.L = rc.NewRC_Listener_m_j(pool.BP, rcaddr, m)
	m.L.Name = "Master"
	if m.HbDelay > 0 {
		m.L.PingDelay = m.HbDelay
	}
	m.L.LCH = m
	m.L.AddToken(ts)
	m.L.RCBH.AddF(ChannelCmdS, m.OnChannelCmd)
	m.L.AddHFunc("dial", m.DailH)
	m.L.AddHFunc("close", m.CloseH)
	m.L.AddHFunc("list", m.ListH)
	err = m.L.Run()
	return
}

func (m *Master) OnLogin(rc *impl.RCM_Cmd, token string) (cid string, err error) {
	name := rc.StrVal("name")
	ctype := rc.StrVal("ctype")
	session := rc.StrVal("session")
	if len(ctype) < 1 {
		err = fmt.Errorf("ctype is required")
	}
	cid, err = m.L.RCH.OnLogin(rc, token)
	if err != nil {
		return
	}
	var old string
	m.slck.Lock()
	if ctype == TypeSlaver {
		old = m.slavers[name]
		m.slavers[name] = cid
	} else if ctype == TypeClient {
		old = m.clients[session]
		m.clients[session] = cid
	} else {
		err = fmt.Errorf("the ctype must be in slaver/client")
		m.slck.Unlock()
		return
	}
	rc.Kvs().SetVal("name", name)
	rc.Kvs().SetVal("ctype", ctype)
	rc.Kvs().SetVal("session", session)
	m.slck.Unlock()
	m.L.AddC_rc(cid, rc)
	oldCmd := m.L.CmdC(old)
	if oldCmd != nil {
		oldCmd.Close()
	}
	if ctype == TypeSlaver {
		log.D("Master accept slaver connect by name(%v) from %v", name, rc.RemoteAddr())
	} else {
		log.D("Master accept client connect by session(%v) from %v", session, rc.RemoteAddr())
	}
	return
}

func (m *Master) DailH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var name, uri string
	err = rc.ValidF(`
		uri,R|S,L:0;
		name,O|S,L:0;
		`, &uri, &name)
	if err != nil {
		return
	}
	m.slck.RLock()
	cid := m.slavers[name]
	m.slck.RUnlock()
	if len(cid) < 1 {
		err = fmt.Errorf("the channel is not found by name(%v)", name)
		return
	}
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		err = fmt.Errorf("the channel is not found by name(%v)", name)
		return
	}
	session := rc.Kvs().StrVal("session")
	log.D("Master try dial to %v on channel(%v),session(%v)", uri, name, session)
	res, err := cmdc.Exec_m("dial", util.Map{
		"uri":  uri,
		"name": name,
	})
	if err != nil {
		return
	}
	sid := uint16(res.IntVal("sid"))

	m.slck.Lock()
	m.ni2s[fmt.Sprintf("%v-%v", name, sid)] = session
	m.si2n[fmt.Sprintf("%v-%v", session, sid)] = name
	m.slck.Unlock()
	val = res
	log.D("Master dial to %v on channel(%v),session(%v) success with sid(%v)", uri, name, session, sid)
	return
}

func (m *Master) ListH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	m.slck.RLock()
	defer m.slck.RUnlock()
	var slavers = util.Map{}
	for name, cid := range m.slavers {
		cmdc := m.L.CmdC(cid)
		if cmdc == nil {
			slavers[name] = "offline"
		} else {
			slavers[name] = "ok->" + cmdc.RemoteAddr().String()
		}
	}
	var clients = util.Map{}
	for session, cid := range m.clients {
		cmdc := m.L.CmdC(cid)
		if cmdc == nil {
			clients[session] = "offline"
		} else {
			clients[session] = "ok->" + cmdc.RemoteAddr().String()
		}
	}
	val = util.Map{
		"slaver": slavers,
		"client": clients,
	}
	return
}

func (m *Master) CloseH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var sid uint16
	err = rc.ValidF(`
		sid,R|S,L:0;
		`, &sid)
	if err != nil {
		return
	}
	session := rc.Kvs().StrVal("session")
	m.slck.RLock()
	name := m.si2n[fmt.Sprintf("%v-%v", session, sid)]
	cid := m.slavers[name]
	m.slck.RUnlock()
	defer func() {
		m.slck.Lock()
		delete(m.ni2s, fmt.Sprintf("%v-%v", name, sid))
		delete(m.si2n, fmt.Sprintf("%v-%v", session, sid))
		m.slck.Unlock()
	}()
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		err = fmt.Errorf("slaver not found")
		log.D("Master close session(%v) on name(%v) fail with %v", sid, name, err)
		return
	}
	val, err = cmdc.Exec_m("close", util.Map{
		"sid": sid,
	})
	if err == nil {
		log.D("Master close session(%v) on name(%v) success", sid, name)
	} else {
		log.D("Master close session(%v) on name(%v) fail with %v", sid, name, err)
	}
	return
}

func (m *Master) OnChannelCmd(c netw.Cmd) int {
	data := c.Data()
	if len(data) < 3 {
		c.Writeb([]byte("data is not correct"))
		return -1
	}
	ctype := c.Kvs().StrVal("ctype")
	if ShowLog > 1 {
		log.D("Master recieve %v data from %v", len(data), ctype)
	}
	if ctype == TypeSlaver {
		return m.OnSlaverCmd(c)
	}
	return m.OnClientCmd(c)
}

func (m *Master) OnSlaverCmd(c netw.Cmd) int {
	name := c.Kvs().StrVal("name")
	data := c.Data()
	sid := binary.BigEndian.Uint16(data[1:])
	m.slck.RLock()
	session := m.ni2s[fmt.Sprintf("%v-%v", name, sid)]
	cid := m.clients[session]
	m.slck.RUnlock()
	if len(session) < 1 {
		c.Writeb([]byte(ErrSessionNotFound.Error()))
		return 0
	}
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		c.Writeb([]byte("client not found"))
		return -1
	}
	reply, err := cmdc.ExecV(ChannelCmdC, true, data)
	if err != nil {
		c.Writeb([]byte(err.Error()))
		log.D("Master client repy error %v", err)
	} else {
		c.Writeb(reply)
	}
	return 0
}

func (m *Master) OnClientCmd(c netw.Cmd) int {
	session := c.Kvs().StrVal("session")
	data := c.Data()
	sid := binary.BigEndian.Uint16(data[1:])
	m.slck.RLock()
	name := m.si2n[fmt.Sprintf("%v-%v", session, sid)]
	cid := m.slavers[name]
	m.slck.RUnlock()
	if len(name) < 1 {
		c.Writeb([]byte(ErrSessionNotFound.Error()))
		return 0
	}
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		c.Writeb([]byte("slaver not found"))
		return -1
	}
	reply, err := cmdc.ExecV(ChannelCmdC, true, c.Data())
	if err != nil {
		c.Writeb([]byte(err.Error()))
		log.D("Master slaver repy error %v", err)
	} else {
		c.Writeb(reply)
	}
	return 0
}

//OnConn see ConHandler for detail
func (m *Master) OnConn(c netw.Con) bool {
	c.SetWait(true)
	return true
}

//OnClose see ConHandler for detail
func (m *Master) OnClose(c netw.Con) {
	m.slck.Lock()
	name := c.Kvs().StrVal("name")
	if len(name) > 0 {
		delete(m.slavers, name)
		log.D("Master the %v connection(%v) is closed", TypeSlaver, name)
	}
	session := c.Kvs().StrVal("session")
	if len(session) > 0 {
		delete(m.clients, session)
		log.D("Master the %v connection(%v) is closed", TypeClient, session)
	}
	m.slck.Unlock()
}

//OnCmd see ConHandler for detail
func (m *Master) OnCmd(c netw.Cmd) int {
	return 0
}

func (m *Master) Wait() {
	m.L.Wait()
}

func (m *Master) Close() (err error) {
	if m.L != nil {
		m.L.Close()
	}
	return
}

type Slaver struct {
	Alias   string
	R       *rc.RC_Runner_m
	SP      *SessionPool
	Channel *Channel
	HbDelay int64
	OnLogin func(a *rc.AutoLoginH, err error)
}

func NewSlaver(alias string) *Slaver {
	return &Slaver{
		Alias: alias,
		SP:    NewSessionPool(),
	}
}

func (s *Slaver) StartSlaver(rcaddr, name, token string) (err error) {
	return s.Start(rcaddr, name, "", token, TypeSlaver)
}

func (s *Slaver) StartClient(rcaddr, session, token string) (err error) {
	return s.Start(rcaddr, "", session, token, TypeClient)
}

func (s *Slaver) Start(rcaddr, name, session, token, ctype string) (err error) {
	auto := rc.NewAutoLoginH(token)
	auto.OnLogin = s.OnLogin
	auto.Args = util.Map{
		"alias":   s.Alias,
		"ctype":   ctype,
		"token":   token,
		"name":    name,
		"session": session,
	}
	s.R = rc.NewRC_Runner_m_j(pool.BP, rcaddr, netw.NewCCH(netw.NewQueueConH(auto, s), s))
	s.R.Name = s.Alias
	auto.Runner = s.R
	s.Channel = NewChannel(s.R.RCBH, s.R.RCM_Con.RC_Con, s.R.RCM_Con, s.R.RCM_S, s.SP)
	s.R.Start()
	if s.HbDelay > 0 {
		s.R.HbDelay = s.HbDelay
		s.R.StartHbTimer()
	}
	return s.R.Valid()
}

func (s *Slaver) DialSession(name, uri string) (session *Session, err error) {
	return s.Channel.DialSession(name, uri)
}

func (s *Slaver) CloseSession(sid uint16) (err error) {
	return s.Channel.Close(sid)
}

func (s *Slaver) List() (res util.Map, err error) {
	return s.Channel.List()
}

//OnConn see ConHandler for detail
func (s *Slaver) OnConn(con netw.Con) bool {
	//fmt.Println("master is connected")
	return true
}

//OnClose see ConHandler for detail
func (s *Slaver) OnClose(con netw.Con) {
	//fmt.Println("master is disconnected")
}

//OnCmd see ConHandler for detail
func (s *Slaver) OnCmd(con netw.Cmd) int {
	return 0
}

func (s *Slaver) Close() error {
	s.R.Stop()
	return nil
}

type Channel struct {
	BH *impl.OBDH
	RC *impl.RC_Con
	RM *impl.RCM_Con
	RS *impl.RCM_S
	SP *SessionPool
}

func NewChannel(bh *impl.OBDH, rc *impl.RC_Con, rm *impl.RCM_Con, rs *impl.RCM_S, sp *SessionPool) *Channel {
	channel := &Channel{
		BH: bh,
		RC: rc,
		RM: rm,
		RS: rs,
		SP: sp,
	}
	channel.RS.AddHFunc("dial", channel.DialH)
	channel.RS.AddHFunc("close", channel.CloseH)
	channel.BH.AddF(ChannelCmdC, channel.OnMasterCmd)
	return channel
}

func (c *Channel) ExecBytes(args []byte) (reply []byte, err error) {
	reply, err = c.RC.ExecV(ChannelCmdS, true, args)
	return
}

func (c *Channel) DialH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var uri string
	err = rc.ValidF(`
		uri,R|S,L:0;
		`, &uri)
	if err != nil {
		return
	}
	session, err := c.SP.Dail(uri, c)
	if err != nil {
		return
	}
	val = util.Map{
		"uri": uri,
		"sid": session.SID,
	}
	log.D("Channel create session by uri(%v) is success with sid(%v)", uri, session.SID)
	return
}

func (c *Channel) CloseH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var sid uint16
	err = rc.ValidF(`
		sid,R|S,L:0;
		`, &sid)
	if err != nil {
		return
	}
	session := c.SP.Remove(sid)
	if session == nil {
		err = fmt.Errorf("session(%v) is not found", sid)
	}
	val = util.Map{
		"code": 0,
		"sid":  session.SID,
	}
	log.D("Channel remove session(%v) success", session.SID)
	return
}

func (c *Channel) Close(sid uint16) (err error) {
	session := c.SP.Find(sid)
	if session == nil {
		err = fmt.Errorf("session(%v) is not exists", sid)
		return
	}
	defer session.Close()
	_, err = c.RM.Exec_m("close", util.Map{
		"sid": sid,
	})
	if err == nil {
		log.D("Channel close session(%v) success", sid)
	} else {
		log.D("Channel close session(%v) fail with %v", sid, err)
	}
	return
}

func (c *Channel) Dial(name, uri string) (sid uint16, err error) {
	res, err := c.RM.Exec_m("dial", util.Map{
		"uri":  uri,
		"name": name,
	})
	if err == nil {
		sid = uint16(res.IntVal("sid"))
		log.D("Channel dial to %v by name(%v) success with sid(%v)", uri, name, sid)
	}
	return
}

func (c *Channel) List() (res util.Map, err error) {
	res, err = c.RM.Exec_m("list", util.Map{})
	return
}

func (c *Channel) Write(p []byte) (n int, err error) {
	reply, err := c.ExecBytes(p)
	if err != nil {
		return
	}
	if len(reply) < 1 {
		log.E("Channel receive empty reply")
		err = ErrSessionClosed
		return
	}
	message := string(reply)
	switch message {
	case ErrSessionClosed.Error():
		err = ErrSessionClosed
	case ErrSessionNotFound.Error():
		err = ErrSessionNotFound
	case OK:
		err = nil
	default:
		err = fmt.Errorf(message)
	}
	n = len(p)
	return
}

func (c *Channel) OnMasterCmd(cmd netw.Cmd) int {
	data := cmd.Data()
	// log.D("Channel receive %v data from %v", len(data), cmd.RemoteAddr())
	_, err := c.SP.Write(data)
	if err == nil {
		cmd.Writev([]byte(OK))
	} else {
		cmd.Writeb([]byte(err.Error()))
	}
	return 0
}

func (c *Channel) DialSession(name, uri string) (session *Session, err error) {
	sid, err := c.Dial(name, uri)
	if err == nil {
		session = c.SP.Start(sid, c)
		log.D("Channel dial to %v on channel(%v) success with %v", uri, name, sid)
	}
	return
}
