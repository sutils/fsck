package fsck

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/Centny/gwf/tutil"

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
	Local   *Slaver
	Forward *Forward
}

func NewServer() *Server {
	var srv = &Server{
		Master: NewMaster(),
		Local:  NewSlaver("local"),
	}
	srv.Forward = NewForward(srv.Master.DialSession)
	return srv
}

func (s *Server) Run(addr string, ts map[string]int) (err error) {
	if ts == nil {
		ts = map[string]int{}
	}
	token := util.UUID()
	ts[token] = 1
	err = s.Master.Run(addr, ts)
	if err == nil {
		err = s.Local.StartSlaver(addr, "master", token)
	}
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
	SP      *SessionPool
	slck    sync.RWMutex
	slavers map[string]string //slaver name map to connect id
	clients map[string]string //client session map to connect id
	ni2s    map[string]string //mapping <name-sid> to session
	si2n    map[string]string //mapping <session-sid> to name
	sidc    uint16
	//
	pings map[uint16]int64
}

func NewMaster() *Master {
	srv := &Master{
		SP:      NewSessionPool(),
		slck:    sync.RWMutex{},
		slavers: map[string]string{},
		clients: map[string]string{},
		ni2s:    map[string]string{},
		si2n:    map[string]string{},
		pings:   map[uint16]int64{},
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
	m.L.AddFFunc("^/usr/.*$", m.AccessH)
	m.L.AddHFunc("/usr/dial", m.DialH)
	m.L.AddHFunc("/usr/close", m.CloseH)
	m.L.AddHFunc("/usr/list", m.ListH)
	m.L.AddHFunc("/usr/status", m.StatusH)
	m.L.AddHFunc("/usr/real_log", m.RealLogH)
	m.L.AddHFunc("ping", m.PingH)
	err = m.L.Run()
	return
}

func (m *Master) AccessH(rc *impl.RCM_Cmd) (bool, interface{}, error) {
	ctype := rc.Kvs().StrVal("ctype")
	if len(ctype) < 1 {
		return false, nil, fmt.Errorf("not login")
	}
	return true, nil, nil
}

func (m *Master) OnLogin(rc *impl.RCM_Cmd, token string) (cid string, err error) {
	name := rc.StrVal("name")
	ctype := rc.StrVal("ctype")
	session := rc.StrVal("session")
	if len(ctype) < 1 {
		err = fmt.Errorf("ctype is required")
		return
	}
	cid, _ = m.L.RCH.OnLogin(rc, token)
	var old string
	m.slck.Lock()
	defer m.slck.Unlock()
	if ctype == TypeSlaver {
		if len(name) < 1 {
			err = fmt.Errorf("name is required for slaver")
			return
		}
		old = m.slavers[name]
		m.slavers[name] = cid
	} else if ctype == TypeClient {
		if len(session) < 1 {
			err = fmt.Errorf("session is required for client")
			return
		}
		old = m.clients[session]
		m.clients[session] = cid
	} else {
		err = fmt.Errorf("the ctype must be in slaver/client")
		return
	}
	rc.Kvs().SetVal("name", name)
	rc.Kvs().SetVal("ctype", ctype)
	rc.Kvs().SetVal("session", session)
	m.L.AddC_rc(cid, rc)
	m.L.CloseC(old)
	if ctype == TypeSlaver {
		log.D("Master accept slaver connect by name(%v) from %v", name, rc.RemoteAddr())
	} else {
		log.D("Master accept client connect by session(%v) from %v", session, rc.RemoteAddr())
	}
	return
}

func (m *Master) StatusH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var ns []string
	err = rc.ValidF(`
		name,R|S,L:0;
		`, &ns)
	if err != nil {
		return
	}
	m.slck.RLock()
	cids := map[string]string{}
	for _, name := range ns {
		cid := m.slavers[name]
		if len(cid) > 0 {
			cids[name] = cid
		}
	}
	m.slck.RUnlock()
	allres := util.Map{}
	for name, cid := range cids {
		cmdc := m.L.CmdC(cid)
		if cmdc == nil {
			allres[name] = util.Map{
				"status": "offline",
			}
			continue
		}
		res, err := cmdc.Exec_m("status", util.Map{})
		if err != nil {
			allres[name] = util.Map{
				"status": err.Error(),
			}
		} else {
			res["status"] = "ok"
			allres[name] = res
		}
	}
	val = allres
	return
}

func (m *Master) RealLogH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var ns []string
	err = rc.ValidF(`
		name,R|S,L:0;
		`, &ns)
	if err != nil {
		return
	}
	m.slck.RLock()
	cids := map[string]string{}
	for _, name := range ns {
		cid := m.slavers[name]
		if len(cid) > 0 {
			cids[name] = cid
		}
	}
	m.slck.RUnlock()
	allres := util.Map{}
	for name, cid := range cids {
		cmdc := m.L.CmdC(cid)
		if cmdc == nil {
			allres[name] = util.Map{
				"status": "offline",
			}
			continue
		}
		res, err := cmdc.Exec_m("real_log", *rc.Map)
		if err != nil {
			allres[name] = util.Map{
				"status": err.Error(),
			}
		} else {
			res["status"] = "ok"
			allres[name] = res
		}
	}
	val = allres
	return
}

func (m *Master) DialH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var name, uri string
	err = rc.ValidF(`
		uri,R|S,L:0;
		name,O|S,L:0;
		`, &uri, &name)
	if err != nil {
		return
	}
	session := rc.Kvs().StrVal("session")
	if len(session) < 1 {
		err = fmt.Errorf("the session is empty, not login?")
		return
	}
	_, val, err = m.Dial(session, name, uri)
	return
}

func (m *Master) DialSession(name, uri string, raw io.WriteCloser) (session Session, err error) {
	sid, _, err := m.Dial("master", name, uri)
	if err == nil {
		session = m.SP.Bind(sid, WriterF(func(p []byte) (n int, err error) {
			_, err = m.WriteToSlaver(name, p)
			n = len(p)
			return
		}), raw)
	}
	return
}

func (m *Master) Dial(session, name, uri string) (sid uint16, res util.Map, err error) {
	m.slck.Lock()
	cid := m.slavers[name]
	m.sidc++
	sid = m.sidc
	m.slck.Unlock()
	if len(cid) < 1 {
		err = fmt.Errorf("the channel is not found by name(%v)", name)
		return
	}
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		err = fmt.Errorf("the channel is not found by name(%v)", name)
		return
	}
	log.D("Master try dial to %v on channel(%v),session(%v)", uri, name, session)
	res, err = cmdc.Exec_m("dial", util.Map{
		"uri":  uri,
		"name": name,
		"sid":  sid,
	})
	if err != nil {
		return
	}
	// sid := uint16(res.IntVal("sid"))
	m.slck.Lock()
	m.ni2s[fmt.Sprintf("%v-%v", name, sid)] = session
	m.si2n[fmt.Sprintf("%v-%v", session, sid)] = name
	if uri == "echo" {
		m.pings[sid] = 0
	}
	m.slck.Unlock()
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
	if len(session) < 1 {
		err = fmt.Errorf("the session is empty, not login?")
		return
	}
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

func (m *Master) PingH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var name string
	err = rc.ValidF(`
		name,R|S,L:0;
		`, &name)
	if err != nil {
		return
	}
	m.slck.RLock()
	cid := m.slavers[name]
	m.slck.RUnlock()
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		err = fmt.Errorf("slaver not found")
		log.D("Master ping slaver(%v) fail with %v", name, err)
		return
	}
	beg := util.Now()
	res, err := cmdc.Exec_m("ping", util.Map{
		"data": rc.Val("data"),
	})
	if err == nil {
		used := util.Now() - beg
		res[name] = used
		val = res
		log.D("Master ping slaver(%v) success by used(%v)", name, time.Duration(used)*time.Millisecond)
	} else {
		log.D("Master ping slaver(%v) fail with %v", name, err)
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

func (m *Master) Send(sid uint16, cid string, data []byte) (reply []byte, err error) {
	cmdc := m.L.CmdC(cid)
	if cmdc == nil {
		log.D("Master transfer data to cid(%v) fail with connect not found", cid)
		err = fmt.Errorf("connection not found by id(%v)", cid)
		return
	}
	m.slck.RLock()
	_, pings := m.pings[sid]
	m.slck.RUnlock()
	//
	if pings {
		beg := util.Now()
		buf := make([]byte, 8)
		binary.BigEndian.PutUint64(buf, uint64(beg))
		reply, err = cmdc.ExecV(ChannelCmdC, true, append(data, buf...))
		reply = append(reply, []byte(fmt.Sprintf(":%v", util.S2Json(util.Map{
			"used": util.Now() - beg,
		})))...)
	} else {
		reply, err = cmdc.ExecV(ChannelCmdC, true, data)
	}
	return
}

func (m *Master) WriteToClient(name string, data []byte) (reply []byte, err error) {
	sid := binary.BigEndian.Uint16(data[1:])
	m.slck.RLock()
	session := m.ni2s[fmt.Sprintf("%v-%v", name, sid)]
	cid := m.clients[session]
	m.slck.RUnlock()
	switch session {
	case "":
		log.D("Master transfer data to client by name(%v),sid(%v) fail with session not found", name, sid)
		err = ErrSessionNotFound
	case "master":
		_, err = m.SP.Write(data)
		if err == nil {
			reply = []byte(OK)
		} else {
			reply = []byte(err.Error())
		}
	default:
		reply, err = m.Send(sid, cid, data)
	}
	return
}

func (m *Master) WriteToSlaver(session string, data []byte) (reply []byte, err error) {
	sid := binary.BigEndian.Uint16(data[1:])
	m.slck.RLock()
	name := m.si2n[fmt.Sprintf("%v-%v", session, sid)]
	cid := m.slavers[name]
	m.slck.RUnlock()
	if len(name) < 1 {
		log.D("Master transfer data to slaver by session(%v),sid(%v) fail with name not found", session, sid)
		err = ErrSessionNotFound
	} else {
		reply, err = m.Send(sid, cid, data)
	}
	return
}

func (m *Master) OnSlaverCmd(c netw.Cmd) int {
	name := c.Kvs().StrVal("name")
	data := c.Data()
	reply, err := m.WriteToClient(name, data)
	if err != nil {
		c.Writeb([]byte(err.Error()))
		log.D("Master slaver repy error %v", err)
	} else {
		c.Writeb(reply)
	}
	return 0
}

func (m *Master) OnClientCmd(c netw.Cmd) int {
	session := c.Kvs().StrVal("session")
	data := c.Data()
	reply, err := m.WriteToSlaver(session, data)
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
	Auto    *rc.AutoLoginH
	OnLogin func(a *rc.AutoLoginH, err error)
	Real    *RealTime
}

func NewSlaver(alias string) *Slaver {
	slaver := &Slaver{
		Alias: alias,
		SP:    NewSessionPool(),
		Real:  NewRealTime(),
	}
	return slaver
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
	s.Auto = auto
	s.R = rc.NewRC_Runner_m_j(pool.BP, rcaddr, netw.NewCCH(netw.NewQueueConH(auto, s), s))
	s.R.Name = s.Alias
	auto.Runner = s.R
	s.Channel = NewChannel(s.R.RCBH, s.R.RCM_Con.RC_Con, s.R.RCM_Con, s.R.RCM_S, s.SP)
	s.Channel.Real = s.Real
	s.Channel.Name = ctype
	s.R.Start()
	if s.HbDelay > 0 {
		s.R.HbDelay = s.HbDelay
		s.R.StartHbTimer()
	}

	return s.R.Valid()
}

func (s *Slaver) DialSession(name, uri string, raw io.WriteCloser) (session Session, err error) {
	return s.Channel.DialSession(name, uri, raw)
}

func (s *Slaver) CloseSession(sid uint16) (err error) {
	return s.Channel.Close(sid)
}

func (s *Slaver) List() (res util.Map, err error) {
	return s.Channel.List()
}

func (s *Slaver) PingExec(name, data string) (used, slaver int64, err error) {
	used, slaver, err = s.Channel.PingExec(name, data)
	return
}

func (s *Slaver) PingSession(name, data string) (used, slaverCall, slaverBack int64, err error) {
	used, slaverCall, slaverBack, err = s.Channel.PingSession(name, data)
	return
}

func (s *Slaver) Status(name ...string) (status util.Map, err error) {
	status, err = s.Channel.Status(name...)
	return
}

func (s *Slaver) RealLog(name []string, ns map[string]int64, keys map[string]string, clear int) (all util.Map, err error) {
	all, err = s.Channel.RealLog(name, ns, keys, clear)
	return
}

//OnConn see ConHandler for detail
func (s *Slaver) OnConn(con netw.Con) bool {
	//fmt.Println("master is connected")
	log.D("Slaver the master is connected")
	return true
}

//OnClose see ConHandler for detail
func (s *Slaver) OnClose(con netw.Con) {
	//fmt.Println("master is disconnected")
	log.D("Slaver the master is disconnected")
}

//OnCmd see ConHandler for detail
func (s *Slaver) OnCmd(con netw.Cmd) int {
	return 0
}

func (s *Slaver) Close() error {
	s.R.Stop()
	s.SP.Close()
	return nil
}

type Channel struct {
	Name  string
	BH    *impl.OBDH
	RC    *impl.RC_Con
	RM    *impl.RCM_Con
	RS    *impl.RCM_S
	SP    *SessionPool
	M     *tutil.Monitor
	pings map[string]*EchoPing
	pslck sync.RWMutex
	Real  *RealTime
}

func NewChannel(bh *impl.OBDH, rc *impl.RC_Con, rm *impl.RCM_Con, rs *impl.RCM_S, sp *SessionPool) *Channel {
	channel := &Channel{
		BH:    bh,
		RC:    rc,
		RM:    rm,
		RS:    rs,
		SP:    sp,
		M:     tutil.NewMonitor(),
		pings: map[string]*EchoPing{},
		pslck: sync.RWMutex{},
		Real:  NewRealTime(),
	}
	channel.RS.AddHFunc("status", channel.StatusH)
	channel.RS.AddHFunc("dial", channel.DialH)
	channel.RS.AddHFunc("close", channel.CloseH)
	channel.RS.AddHFunc("ping", channel.PingH)
	channel.RS.AddHFunc("real_log", channel.RealLogH)
	channel.BH.AddF(ChannelCmdC, channel.OnMasterCmd)
	return channel
}

func (c *Channel) ExecBytes(args []byte) (reply []byte, err error) {
	defer c.M.Done(c.M.Start("exec_bytes"))
	reply, err = c.RC.ExecV(ChannelCmdS, true, args)
	return
}

func (c *Channel) DialH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	var uri string
	var sid uint16
	err = rc.ValidF(`
		uri,R|S,L:0;
		sid,R|I,R:0;
		`, &uri, &sid)
	if err != nil {
		return
	}
	defer c.M.Done(c.M.Start("dial"))
	session, err := c.SP.Dial(sid, uri, c)
	if err != nil {
		return
	}
	val = util.Map{
		"uri": uri,
		"sid": session.ID(),
	}
	log.D("Channel(%v) create session by uri(%v) is success with sid(%v)", c.Name, uri, session.ID())
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
	defer c.M.Done(c.M.Start("close"))
	session := c.SP.Remove(sid)
	if session == nil {
		err = fmt.Errorf("session(%v) is not found", sid)
		return
	}
	val = util.Map{
		"code": 0,
		"sid":  session.ID(),
	}
	log.D("Channel(%v) remove session(%v) success", c.Name, session.ID())
	return
}

func (c *Channel) Close(sid uint16) (err error) {
	session := c.SP.Find(sid)
	if session == nil {
		err = fmt.Errorf("local session(%v) is not exists", sid)
		return
	}
	defer session.Close()
	_, err = c.RM.Exec_m("/usr/close", util.Map{
		"sid": sid,
	})
	if err == nil {
		log.D("Channel(%v) close remote session(%v) success", c.Name, sid)
	} else {
		log.D("Channel(%v) close remote session(%v) fail with %v", c.Name, sid, err)
	}
	return
}

func (c *Channel) Dial(name, uri string) (sid uint16, err error) {
	res, err := c.RM.Exec_m("/usr/dial", util.Map{
		"uri":  uri,
		"name": name,
	})
	if err == nil {
		sid = uint16(res.IntVal("sid"))
		log.D("Channel(%v) dial to %v by name(%v) success with sid(%v)", c.Name, uri, name, sid)
	}
	return
}

func (c *Channel) List() (res util.Map, err error) {
	res, err = c.RM.Exec_m("/usr/list", util.Map{})
	return
}

func (c *Channel) PingH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	val = util.Map{
		"data": rc.Val("data"),
	}
	return
}

func (c *Channel) PingExec(name, data string) (used, slaver int64, err error) {
	beg := util.Now()
	res, err := c.RM.Exec_m("ping", util.Map{
		"data": data,
		"name": name,
	})
	if err == nil {
		slaver = res.IntValV(name, 0)
	}
	used = util.Now() - beg
	return
}

func (c *Channel) Write(p []byte) (n int, err error) {
	reply, err := c.ExecBytes(p)
	if err == nil {
		message := string(reply)
		switch message {
		case ErrSessionClosed.Error():
			err = ErrSessionClosed
		case ErrSessionNotFound.Error():
			err = ErrSessionNotFound
		case OK:
			err = nil
		default:
			if strings.HasPrefix(message, OK+":") {
				err = &ErrOK{Data: strings.TrimPrefix(message, OK+":")}
			} else {
				err = fmt.Errorf(message)
			}
		}
		n = len(p)
	}
	return
}

func (c *Channel) OnMasterCmd(cmd netw.Cmd) int {
	defer c.M.Done(c.M.Start("master_cmd"))
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

func (c *Channel) DialSession(name, uri string, raw io.WriteCloser) (session Session, err error) {
	sid, err := c.Dial(name, uri)
	if err == nil {
		session = c.SP.Bind(sid, c, raw)
		log.D("Channel(%v) dial to %v on channel(%v) success with %v", c.Name, uri, name, sid)
	}
	return
}

func (c *Channel) PingSession(name, data string) (used, slaverCall, slaverBack int64, err error) {
	c.pslck.Lock()
	defer c.pslck.Unlock()
	pings := c.pings[name]
	for i := 0; i < 3; i++ {
		if pings == nil {
			var ss Session
			ss, err = c.DialSession(name, "echo", nil)
			if err == nil {
				pings = NewEchoPing(ss)
				c.pings[name] = pings
			}
		}
		if err == nil {
			used, slaverCall, slaverBack, err = pings.Ping(data)
			if err == io.EOF {
				c.pings[name] = nil
				continue
			} else {
				break
			}
		}
	}
	return
}

func (c *Channel) StatusH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	val, err = c.M.State()
	return
}

func (c *Channel) Status(name ...string) (status util.Map, err error) {
	status, err = c.RM.Exec_m("/usr/status", util.Map{
		"name": strings.Join(name, ","),
	})
	return
}

func (c *Channel) RealLogH(rc *impl.RCM_Cmd) (val interface{}, err error) {
	if rc.IntVal("clear") == 1 {
		c.Real.Clear()
	}
	keys := map[string]string{}
	keysm := rc.MapVal("keys")
	for key := range keysm {
		keys[key] = keysm.StrVal(key)
	}
	ns := map[string]int64{}
	nsm := rc.MapVal("ns")
	for n := range nsm {
		ns[n] = nsm.IntVal(n)
	}
	hosts, alllog := c.Real.MergeLog(ns, keys)
	val = util.Map{"hosts": hosts, "logs": alllog}
	return
}

func (c *Channel) RealLog(name []string, ns map[string]int64, keys map[string]string, clear int) (all util.Map, err error) {
	all, err = c.RM.Exec_m("/usr/real_log", util.Map{
		"name":  strings.Join(name, ","),
		"ns":    ns,
		"keys":  keys,
		"clear": clear,
	})
	return
}
