package main

import (
	"C"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"container/list"
	"encoding/json"
	"io/ioutil"
	"log"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/sutils/fsck"

	"github.com/Centny/gwf/util"
)

const (
	KeyWebCmdURL = "_web_cmd_url"
	WebCmdPrefix = "-sctrlweb:"
)

var SpaceRegex = regexp.MustCompile("[ \n]+")
var CharTerm = []byte{3}
var CharESC = []byte{27}
var CharCtrlb = []byte{6}
var CharEnter = []byte{13}
var CharDelete = []byte{127}

var (
	KeyF1  = []byte{27, 79, 80}
	KeyF2  = []byte{27, 79, 81}
	KeyF3  = []byte{27, 79, 82}
	KeyF4  = []byte{27, 79, 83}
	KeyF5  = []byte{27, 91, 49, 53, 126}
	KeyF6  = []byte{27, 91, 49, 55, 126}
	KeyF7  = []byte{27, 91, 49, 56, 126}
	KeyF8  = []byte{27, 91, 49, 57, 126}
	KeyF9  = []byte{27, 91, 49, 48, 126}
	KeyF10 = []byte{27, 91, 49, 49, 126}
)

type Shell interface {
	io.Writer
	Add(io.Writer)
	Remove(io.Writer)
	String() string
}

type Task struct {
	ID       string
	Subs     map[string]string
	reader   io.ReadCloser
	writer   io.WriteCloser
	errc     int
	Selected []string
	Fields   map[string]string
}

func NewTask(id string) *Task {
	r, w := io.Pipe()
	task := &Task{
		ID:     id,
		Subs:   map[string]string{},
		reader: r,
		writer: w,
		Fields: map[string]string{},
	}
	task.Fields["tid"] = task.ID
	return task
}

func (t *Task) Read(p []byte) (n int, err error) {
	return t.reader.Read(p)
}

func (t *Task) Write(p []byte) (n int, err error) {
	return t.writer.Write(p)
}

func (t *Task) Close() error {
	if t.errc > 0 {
		fmt.Fprintf(t, "%vhaving %v fair\n", WebCmdPrefix, t.errc)
	} else {
		fmt.Fprintf(t, "%vok\n", WebCmdPrefix)
	}
	t.reader.Close()
	t.writer.Close()
	return nil
}

func (t *Task) Header() map[string]string {
	return t.Fields
}

type WebServer struct {
	URL string
	srv *http.Server
	Mux *http.ServeMux
}

func (w *WebServer) Start() (err error) {
	w.srv = &http.Server{Addr: ":0", Handler: w.Mux}
	l, err := net.Listen("tcp", ":0")
	if err == nil {
		parts := strings.Split(l.Addr().String(), ":")
		w.URL = fmt.Sprintf("http://127.0.0.1:%v", parts[len(parts)-1])
		go w.srv.Serve(l)
	}
	return
}

func (w *WebServer) Close() (err error) {
	if w.srv != nil {
		err = w.srv.Close()
	}
	return
}

type Terminal struct {
	// ss        map[string]*SshSession
	ss           *list.List
	slck         sync.RWMutex
	Cmd          *Cmd
	Web          *Web
	Log          *WebLogger
	last         string
	running      bool
	C            *fsck.Slaver
	Mux          *http.ServeMux
	WebSrv       *WebServer
	Forward      *fsck.Forward
	WebCmd       string //the web cmd path
	CmdPrefix    string
	InstancePath string
	Name         string
	Env          []string
	stdout       *os.File
	//
	selected  []string
	activited Shell
	callback  chan []byte
	quit      chan int
	//
	idxc    int
	tidc    uint32
	tasks   map[string]*Task
	taskLck sync.RWMutex
	profile *bytes.Buffer
	//
	keyin   chan []byte
	keydone chan int
	ctrlc   chan os.Signal
	//
	NotTaskCallback chan string
	//
	closed bool
	//
	pings map[string]string
	pslck sync.RWMutex
}

func NewTerminal(c *fsck.Slaver, name, ps1, shell, webcmd string, buffered int) *Terminal {
	term := &Terminal{
		// ss:        map[string]*SshSession{},
		ss:           list.New(),
		slck:         sync.RWMutex{},
		C:            c,
		Mux:          http.NewServeMux(),
		Cmd:          NewCmd("sctrl", ps1, shell),
		Web:          NewWeb(nil),
		Log:          NewWebLogger(name, buffered),
		WebCmd:       webcmd,
		CmdPrefix:    "-sctrl: ",
		callback:     make(chan []byte, 100),
		tasks:        map[string]*Task{},
		taskLck:      sync.RWMutex{},
		InstancePath: "/tmp/.sctrl_instance.json",
		Name:         name,
		Forward:      fsck.NewForward(c),
		stdout:       os.Stdout,
		profile:      bytes.NewBuffer(nil),
		//
		keyin:   make(chan []byte, 10240),
		keydone: make(chan int),
		ctrlc:   make(chan os.Signal, 1),
		//
		pings: map[string]string{},
		pslck: sync.RWMutex{},
	}
	term.Web.H = term.OnWebCmd
	term.WebSrv = &WebServer{Mux: term.Mux}
	term.Mux.Handle("/exec", term.Web)
	term.Mux.HandleFunc("/log", term.Log.WebLogH)
	term.Mux.HandleFunc("/lslog", term.Log.ListLogH)
	prefix := bytes.NewBuffer(nil)
	fmt.Fprintf(prefix, "set +o history\n")
	fmt.Fprintf(prefix, "alias srun='%v/sctrl -run'\n", webcmd)
	fmt.Fprintf(prefix, "alias sexec='%v/sctrl -run sexec'\n", webcmd)
	fmt.Fprintf(prefix, "alias sadd='%v/sctrl -run sadd'\n", webcmd)
	fmt.Fprintf(prefix, "alias srm='%v/sctrl -run srm'\n", webcmd)
	fmt.Fprintf(prefix, "alias sall='%v/sctrl -run sall'\n", webcmd)
	fmt.Fprintf(prefix, "alias spick='%v/sctrl -run spick'\n", webcmd)
	fmt.Fprintf(prefix, "alias shelp='%v/sctrl -run shelp'\n", webcmd)
	fmt.Fprintf(prefix, "alias seval='%v/sctrl -run seval'\n", webcmd)
	fmt.Fprintf(prefix, "alias saddmap='%v/sctrl -run saddmap'\n", webcmd)
	fmt.Fprintf(prefix, "alias srmmap='%v/sctrl -run srmmap'\n", webcmd)
	fmt.Fprintf(prefix, "alias slsmap='%v/sctrl -run slsmap'\n", webcmd)
	fmt.Fprintf(prefix, "alias smaster='%v/sctrl -run smaster'\n", webcmd)
	fmt.Fprintf(prefix, "alias sslaver='%v/sctrl -run sslaver'\n", webcmd)
	fmt.Fprintf(prefix, "alias sreal='%v/sctrl -run sreal'\n", webcmd)
	fmt.Fprintf(prefix, "alias sprofile='%v/sctrl -run profile'\n", webcmd)
	fmt.Fprintf(prefix, "alias sscp='%v/sctrl-scp'\n", webcmd)
	fmt.Fprintf(prefix, "alias sping='%v/sctrl -run sping'\n", webcmd)
	fmt.Fprintf(prefix, "history -d `history 1`\n")
	fmt.Fprintf(prefix, "set -o history\n")
	fmt.Fprintf(prefix, "\n\n")
	term.Cmd.Prefix = prefix
	term.Log.LsName = term.ListLogName
	return term
}

func (t *Terminal) ListLogName() (ns []string) {
	for em := t.ss.Front(); em != nil; em = em.Next() {
		ns = append(ns, em.Value.(*SshSession).Name)
	}
	return
}

func (t *Terminal) OnWebCmd(w *Web, line string) (data interface{}, err error) {
	line = strings.TrimSpace(line)
	log.Printf("Terminal exec command:%v", line)
	cmds := SpaceRegex.Split(line, 2)
	switch cmds[0] {
	case "sall":
		buf := bytes.NewBuffer(nil)
		for em := t.ss.Front(); em != nil; em = em.Next() {
			session := em.Value.(*SshSession)
			if len(t.selected) < 1 || Having(t.selected, session.Name) {
				fmt.Fprintf(buf, "%v:%v\n", session.Name, session.Running)
			}
		}
		data = buf.Bytes()
		return
	case "sadd":
		if len(cmds) < 2 {
			err = saddUsage
			return
		}
		args := SpaceRegex.Split(cmds[1], 3)
		if len(args) < 2 {
			err = saddUsage
			return
		}
		err = t.AddSession(args[0], args[1], len(args) > 2 && args[2] == "connect", nil)
		if err == nil {
			data = fmt.Sprintf("sadd %v success\n", args[0])
		} else {
			err = fmt.Errorf("sadd %v fail with %v", args[0], err)
		}
		t.NotifyTitle()
		return
	case "srm":
		if len(cmds) < 2 {
			err = srmUsage
			return
		}
		buf := bytes.NewBuffer(nil)
		for _, name := range SpaceRegex.Split(cmds[1], -1) {
			var found *list.Element
			for em := t.ss.Front(); em != nil; em = em.Next() {
				session := em.Value.(*SshSession)
				if session.Name == name {
					found = em
					break
				}
				session = nil
			}
			if found == nil {
				fmt.Fprintf(buf, "-error: session %v not found\n", name)
			} else {
				found.Value.(*SshSession).Close()
				t.ss.Remove(found)
				fmt.Fprintf(buf, "-done: session %v remove success\n", name)
			}
		}
		data = buf.Bytes()
		t.NotifyTitle()
		return
	case "spick":
		if len(cmds) < 2 {
			err = spickUsage
			return
		}
		args := SpaceRegex.Split(cmds[1], -1)
		if args[0] == "all" {
			t.selected = []string{}
			data = []byte("ok")
			return
		}
		t.selected = []string{}
		buf := bytes.NewBuffer(nil)
		for _, name := range args {
			var found *list.Element
			for em := t.ss.Front(); em != nil; em = em.Next() {
				session := em.Value.(*SshSession)
				if session.Name == name {
					found = em
					break
				}
				session = nil
			}
			if found != nil {
				t.selected = append(t.selected, name)
				fmt.Fprintf(buf, "-done: session %v is picked\n", name)
			} else {
				fmt.Fprintf(buf, "-error: session %v not found, skipped\n", name)
			}
		}
		data = buf.Bytes()
		return
	case "sexec":
		if len(cmds) < 2 {
			err = sexeclUsage
			return
		}
		task := NewTask(fmt.Sprintf("t%v", atomic.AddUint32(&t.tidc, 1)))
		task.Selected = t.selected
		data = task
		go t.remoteExecf(task, nil, "%v", cmds[1])
		return
	case "sterm":
		if len(cmds) < 2 {
			err = fmt.Errorf("task id is required")
			return
		}
		task := NewTask(fmt.Sprintf("t%v", atomic.AddUint32(&t.tidc, 1)))
		task.Selected = t.selected
		data = task
		go t.remoteTerm(task, cmds[1])
		return
	case "seval":
		if len(cmds) < 2 {
			err = sevalUsage
			return
		}
		args := strings.TrimSpace(cmds[1])
		var scriptPath, scriptArgs string
		if strings.HasPrefix(args, "\"") {
			parts := strings.SplitN(args, "\"", 3)
			if len(parts) < 2 {
				err = fmt.Errorf("%v having bash \"", args)
				return
			}
			scriptPath = parts[1]
			scriptArgs = strings.TrimSpace(parts[2])
		} else {
			parts := SpaceRegex.Split(args, 2)
			scriptPath = parts[0]
			if len(parts) > 1 {
				scriptArgs = parts[1]
			}
		}
		var scriptBytes []byte
		scriptBytes, err = ioutil.ReadFile(scriptPath)
		if err != nil {
			return
		}
		task := NewTask(fmt.Sprintf("t%v", atomic.AddUint32(&t.tidc, 1)))
		task.Selected = t.selected
		data = task
		go t.remoteExecf(task, scriptBytes, "%v", scriptArgs)
		return
	case "saddmap":
		if len(cmds) < 2 {
			err = saddmapUsage
			return
		}
		args := SpaceRegex.Split(cmds[1], 3)
		if len(args) < 2 {
			err = saddmapUsage
			return
		}
		var m *fsck.Mapping
		if len(args) > 2 {
			m = &fsck.Mapping{
				Name:   args[0],
				Local:  args[1],
				Remote: args[2],
			}
		} else {
			m = &fsck.Mapping{
				Name:   args[0],
				Remote: args[1],
			}
		}
		_, err = t.Forward.Start(m)
		if err == nil {
			data = fmt.Sprintf("%v mapping %v to %v success\n", m.Name, m.Local, m.Remote)
		}
		return
	case "srmmap":
		if len(cmds) < 2 {
			err = srmmapUsage
			return
		}
		err = t.Forward.Stop(cmds[1], len(cmds) > 2 && cmds[2] == "connected")
		data = "ok\n"
		return
	case "slsmap":
		buf := bytes.NewBuffer(nil)
		var namemax, localmax int
		for _, m := range t.Forward.List() {
			namelen := len(m.Name)
			if namemax < namelen {
				namemax = namelen
			}
			locallen := len(m.Local)
			if localmax < locallen {
				localmax = locallen
			}
		}
		var format = fmt.Sprintf(" %v%v%v %v%v%v %v\n", "%", namemax, "s", "%", localmax, "s", "%v")
		for _, m := range t.Forward.List() {
			fmt.Fprintf(buf, format, m.Name, m.Local, m.Remote)
		}
		data = buf.Bytes()
	case "smaster":
		var res util.Map
		res, err = t.C.List()
		if err == nil {
			buf := bytes.NewBuffer(nil)
			slaver := res.MapVal("slaver")
			fmt.Fprintf(buf, "Slaver:\n")
			for name, status := range slaver {
				fmt.Fprintf(buf, "  %10s   %v\n", name, status)
			}
			fmt.Fprintf(buf, "\n")
			client := res.MapVal("client")
			fmt.Fprintf(buf, "Client:\n")
			for session, status := range client {
				fmt.Fprintf(buf, "  %24s   %v\n", session, status)
			}
			fmt.Fprintf(buf, "\n")
			data = buf.Bytes()
		}
		return
	case "sslaver":
		if len(cmds) < 2 {
			err = sslaverUsage
			return
		}
		args := SpaceRegex.Split(cmds[1], -1)
		var allres util.Map
		allres, err = t.C.Status(args...)
		if err == nil {
			buf := bytes.NewBuffer(nil)
			for name := range allres {
				res := allres.MapVal(name)
				fmt.Fprintf(buf, "Slaver %v: %v\n", name, res.Val("status"))
				pending := res.MapVal("pending")
				if pending != nil {
					fmt.Fprintf(buf, "  %-10s -> %v\n", "pending", len(pending))
				}
				used := res.AryMapVal("used")
				for _, action := range used {
					fmt.Fprintf(buf, "  %-10s -> avg:%-3d max:%-5d count:%-4d\n", action.StrVal("name"),
						action.IntVal("avg"), action.IntVal("max"), action.IntVal("count"))
				}
				fmt.Fprintf(buf, "\n")
			}
			data = buf.Bytes()
		}
	case "sreal":
		if len(cmds) < 2 {
			err = srealUsage
			return
		}
		args := SpaceRegex.Split(cmds[1], -1)
		var timeout, delay int64 = 0, 2
		ns := map[string]int64{}
		keys := map[string]string{}
		name := ""
		clear := 0
		for idx, arg := range args {
			if idx == 0 {
				name = arg
			} else if strings.HasPrefix(arg, "-clear") {
				clear = 1
			} else if strings.HasPrefix(arg, "-host=") {
				parts := strings.Split(strings.TrimPrefix(arg, "-host="), ",")
				for _, part := range parts {
					ns[part] = 0
				}
			} else if strings.HasPrefix(arg, "-timeout=") {
				timeout, _ = strconv.ParseInt(strings.TrimPrefix(arg, "-timeout="), 10, 64)
			} else if strings.HasPrefix(arg, "-delay=") {
				delay, _ = strconv.ParseInt(strings.TrimPrefix(arg, "-delay="), 10, 64)
			} else {
				parts := strings.SplitN(arg, "=", 2)
				if len(parts) > 1 {
					keys[parts[0]] = parts[1]
				} else {
					keys[parts[0]] = "sum"
				}
			}
		}
		for n := range ns {
			ns[n] = timeout * 1000
		}
		if delay < 1 || clear > 0 {
			delay = 0
		}
		//
		task := NewTask("")
		task.Selected = t.selected
		data = task
		go t.execRealTask(task, name, ns, keys, clear, time.Duration(delay)*time.Second)
	case "sping":
		if len(cmds) < 2 {
			err = srmmapUsage
			return
		}
		delay := time.Second
		args := SpaceRegex.Split(cmds[1], 2)
		if len(args) > 1 {
			val, _ := strconv.Atoi(args[1])
			if val > 0 {
				delay = time.Duration(val) * time.Second
			}
		}
		task := NewTask("")
		task.Selected = t.selected
		data = task
		go t.execPingTask(task, args[0], delay)
	case "shelp":
		fallthrough
	case "":
		data = shelpUsage
		return
	case "wssh":
		if len(cmds) < 2 {
			err = fmt.Errorf("echo 'name is reqied' && exit 1")
			return
		}
		var session *SshSession
		var mapping *fsck.Mapping
		session, mapping, err = t.checkHostForward(cmds[1])
		if err == nil {
			suri := fmt.Sprintf("%v@localhost -p %v", session.Username, strings.TrimPrefix(mapping.Local, ":"))
			buf := bytes.NewBuffer(nil)
			fmt.Fprintf(buf, "echo -e \"Sctrl start dial to %v,%v by\\n    uri: %v\\n  cmds: $args\"", session.Name, session.URI, suri)
			fmt.Fprintf(buf, "&& sshpass -p \"%v\" ssh -o StrictHostKeyChecking=no %v $sargs\n", session.Password, suri)
			//fmt.Fprintf(buf, "echo dail to %v,%v by %v\n", session.Name, session.URI, suri)
			data = buf.Bytes()
		}
	case "wscp":
		if len(cmds) < 2 {
			err = fmt.Errorf("echo 'name is reqied' && exit 1")
			return
		}
		args := SpaceRegex.Split(cmds[1], 2)
		if len(args) < 2 {
			err = fmt.Errorf("echo 'two path is required' && exit 1")
			return
		}
		var name, src, dst string
		var upload bool
		if suri := strings.SplitN(args[0], ":", 2); len(suri) > 1 {
			name = strings.TrimSpace(suri[0])
			src = strings.TrimSpace(suri[1])
			if len(name) < 1 || len(src) < 1 {
				err = fmt.Errorf("echo 'souce path invalid(%v)' && exit 1", args[0])
				return
			}
			dst = args[1]
			upload = false
		} else if suri := strings.SplitN(args[1], ":", 2); len(suri) > 1 {
			name = strings.TrimSpace(suri[0])
			src = args[0]
			dst = strings.TrimSpace(suri[1])
			if len(name) < 1 || len(dst) < 1 {
				err = fmt.Errorf("echo 'destination path invalid(%v)' && exit 1", args[1])
				return
			}
			upload = true
		} else {
			err = fmt.Errorf("echo 'source and destionation is not contain host, must have one' && exit 1")
			return
		}
		var session *SshSession
		var mapping *fsck.Mapping
		session, mapping, err = t.checkHostForward(name)
		if err == nil {
			buf := bytes.NewBuffer(nil)
			var suri string
			if upload {
				suri = fmt.Sprintf("%v@localhost:%v", session.Username, dst)
				fmt.Fprintf(buf, "echo -e \"Sctrl start dial to %v,%v by\\n    uri: %v\\n  cmds: scp\"", session.Name, session.URI, suri)
				fmt.Fprintf(buf, "&& sshpass -p \"%v\" scp -o StrictHostKeyChecking=no -P %v -r %v %v\n",
					session.Password, strings.TrimPrefix(mapping.Local, ":"), src, suri)
			} else {
				suri = fmt.Sprintf("%v@localhost:%v", session.Username, src)
				fmt.Fprintf(buf, "echo -e \"Sctrl start dial to %v,%v by\\n    uri: %v\\n  cmds: scp\"", session.Name, session.URI, suri)
				fmt.Fprintf(buf, "&& sshpass -p \"%v\" scp -o StrictHostKeyChecking=no -P %v -r %v %v\n",
					session.Password, strings.TrimPrefix(mapping.Local, ":"), suri, dst)
			}
			//fmt.Fprintf(buf, "echo dail to %v,%v by %v\n", session.Name, session.URI, suri)
			data = buf.Bytes()
		}
	case "profile":
		buf := bytes.NewBuffer(nil)
		for _, env := range t.Env {
			fmt.Fprintf(buf, "%v\n", env)
		}
		fmt.Fprintf(buf, "%v=%v\n", KeyWebCmdURL, t.WebSrv.URL)
		for _, m := range t.Forward.List() {
			fmt.Fprintf(buf, "%v_local=%v\n", m.Name, m.Local)
		}
		for s := t.ss.Front(); s != nil; s = s.Next() {
			one := s.Value.(*SshSession)
			for _, env := range one.Env {
				fmt.Fprintf(buf, "%v_%v\n", one.Name, env)
			}
		}
		fmt.Fprintf(buf, "alias spick='%v spick'\n", "sctrl-exec")
		fmt.Fprintf(buf, "alias sexec='%v sexec'\n", "sctrl-exec")
		fmt.Fprintf(buf, "alias seval='%v seval'\n", "sctrl-exec")
		data = buf.Bytes()
	default:
		err = fmt.Errorf("-error: command %v not found", line)
	}
	return
}

func (t *Terminal) execRealTask(task *Task, name string, ns map[string]int64, keys map[string]string, clear int, delay time.Duration) {
	for {
		allres, err := t.C.RealLog([]string{name}, ns, keys, clear)
		if err == nil {
			res := allres.MapVal(name)
			logs := res.MapVal("logs")
			hosts := res.MapVal("hosts")
			max := len("status")
			for key := range logs {
				if len(key) > max {
					max = len(key)
				}
			}
			fmt.Fprintf(task, "->Slaver %v %v hosts -> %v\n", name, len(hosts), res.Val("status"))
			if len(logs) > 0 {
				vals := []string{}
				for key, val := range logs {
					vals = append(vals, fmt.Sprintf("%v:%v", key, val))
				}
				sort.Sort(util.NewStringSorter(vals))
				buf := ColumnBytes(" ", vals...)
				buf.WriteTo(task)
				_, err = fmt.Fprintf(task, "\n\n")
			}
		} else {
			_, err = fmt.Fprintf(task, "->Slaver %v -> %v\n", name, err)
		}
		if err != nil || delay < 1 {
			break
		}
		time.Sleep(delay)
	}
	task.Close()
}

func (t *Terminal) execPingTask(task *Task, name string, delay time.Duration) {
	data := "1234567890qwertyuiopasdfghjklzxcvbnm"
	for {
		used, call, back, err := t.C.PingSession(name, data)
		if err == nil {
			_, err = fmt.Fprintf(task, "%v bytes from %v: time=%v slaver=(%v,%v)\n", len(data), name,
				time.Duration(used)*time.Millisecond, time.Duration(call)*time.Millisecond,
				time.Duration(back)*time.Millisecond)
		} else {
			_, err = fmt.Fprintf(task, "ping to %v fail with %v\n", name, err)
		}
		if err != nil {
			break
		}
		time.Sleep(delay)
	}
	task.Close()
}

func (t *Terminal) checkHostForward(name string) (session *SshSession, mapping *fsck.Mapping, err error) {
	//check session
	for s := t.ss.Front(); s != nil; s = s.Next() {
		one := s.Value.(*SshSession)
		if one.Name == name {
			session = one
			break
		}
	}
	if session == nil {
		err = fmt.Errorf("echo 'session is not found by name(%v)' && exit 1", name)
		return
	}
	//check forward
	muri := fmt.Sprintf("%v://%v", session.Channel, session.URI)
	allms := t.Forward.List()
	for _, m := range allms {
		if m.Remote == muri {
			mapping = m
			break
		}
	}
	if mapping == nil {
		var name string
		for i := 0; i < 100; i++ {
			if i < 1 {
				name = session.Name
			} else {
				name = fmt.Sprintf("%v-%v", session.Name, i)
			}
			for _, m := range allms {
				if m.Name == name {
					name = ""
					break
				}
			}
			if len(name) > 0 {
				break
			}
		}
		if len(name) < 1 {
			err = fmt.Errorf("echo 'too many forward by name(%v)' && exit 1", name)
			return
		}
		mapping = &fsck.Mapping{
			Name:   name,
			Remote: muri,
		}
		_, err = t.Forward.Start(mapping)
	}
	return
}

func (t *Terminal) remoteTerm(task *Task, tid string) {
	t.taskLck.Lock()
	defer t.taskLck.Unlock()
	termTask := t.tasks[tid]
	if termTask == nil {
		fmt.Fprintf(task, "-fail: task not found by id(%v)\n", tid)
		task.Close()
		return
	}
	log.Printf("Terminal will kill task(%v)\n", tid)
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		name := session.Name
		if len(termTask.Selected) > 0 && !Having(termTask.Selected, name) {
			continue
		}
		if !session.Running {
			fmt.Fprintf(task, "%v->session is not runnning(kill skipped)\n", session.Name)
			continue
		}
		sid := fmt.Sprintf("%v-%v", tid, name)
		if _, ok := termTask.Subs[sid]; !ok {
			fmt.Fprintf(task, "%v->task is already done(kill skipped)\n", session.Name)
			continue
		}
		_, err := session.Write(CharTerm)
		if err == nil {
			fmt.Fprintf(task, "%v->send kill signal success\n", session.Name)
		} else {
			fmt.Fprintf(task, "%v->send kill signal fail with %v\n", session.Name, err)
		}
		termTask.errc++
	}
	termTask.Close()
	task.Close()
}

func (t *Terminal) remoteExecf(task *Task, script []byte, format string, args ...interface{}) {
	cmds := fmt.Sprintf(format, args...)
	log.Printf("remote execute->%v", cmds)
	t.taskLck.Lock()
	t.slck.RLock()
	defer func() {
		t.slck.RUnlock()
		t.taskLck.Unlock()
	}()
	if t.ss.Len() < 1 {
		fmt.Fprintf(task, "no session to exec cmds\n")
		task.Close()
		return
	}
	wg := &sync.WaitGroup{}
	var execSession = func(session *SshSession, sid string) {
		defer wg.Done()
		name := session.Name
		var err error
		tcmds := bytes.NewBuffer(nil)
		fmt.Fprintf(tcmds, "_sexec_sid=%v\n", sid)
		if len(script) > 0 {
			spath := fmt.Sprintf("/tmp/sctrl-%v.sh", sid)
			//
			//prepare remote command list
			fmt.Fprintf(tcmds, "/tmp/sctrl-$_sexec_sid.sh %v\n", cmds)
			fmt.Fprintf(tcmds, "_sexec_code=$?\n")
			fmt.Fprintf(tcmds, "rm -f /tmp/sctrl-$_sexec_sid.sh\n")
			fmt.Fprintf(tcmds, "echo \"%v%v-$_sexec_code\"\n", t.CmdPrefix, sid)
			//
			fmt.Fprintf(task, "%v->start upload script to %v\n", name, spath)
			err = session.UploadScript(spath, script, task)
			if err != nil {
				fmt.Fprintf(task, "%v->upload script to /tmp/sctrl-%v.sh fail with %v\n", name, sid, err)
				task.errc++
				return
			}
			//
			fmt.Fprintf(task, "%v->exec %v %v\n", name, spath, cmds)
		} else {
			//
			//prepare remote command list
			fmt.Fprintf(tcmds, "%v\n", cmds)
			fmt.Fprintf(tcmds, "_sexec_code=$?\n")
			fmt.Fprintf(tcmds, "echo \"%v%v-$_sexec_code\"\n", t.CmdPrefix, sid)
			//
			fmt.Fprintf(task, "%v->exec %v\n", name, cmds)
		}
		_, err = tcmds.WriteTo(session)
		if err == nil {
			task.Subs[sid] = name
			fmt.Fprintf(task, "%v->start success\n", name)
		} else {
			fmt.Fprintf(task, "%v->start fail with %v\n", name, err)
			task.errc++
		}
	}
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		name := session.Name
		if len(task.Selected) < 1 || Having(task.Selected, name) {
			wg.Add(1)
			sid := fmt.Sprintf("%v-%v", task.ID, name)
			go execSession(session, sid)
		}
	}
	wg.Wait()
	if len(task.Subs) < 1 {
		task.Close()
	}
	t.tasks[task.ID] = task
}

func (t *Terminal) handleMessage(message string) {
	log.Printf("handle message->%v", message)
	message = strings.TrimSpace(message)
	parts := SpaceRegex.Split(message, 2)
	back := ""
	if len(parts) > 1 {
		back = parts[1]
	}
	idcode := strings.SplitN(parts[0], "-", 3)
	t.taskLck.Lock()
	task := t.tasks[idcode[0]]
	defer func() {
		if task != nil && len(task.Subs)-task.errc < 1 {
			task.Close()
		}
		t.taskLck.Unlock()
	}()
	if task == nil {
		log.Printf("the receive invalid call back %v, the task not found", message)
		if t.NotTaskCallback != nil {
			t.NotTaskCallback <- message
		}
		return
	}
	log.Printf("do task(%v) on message->%v", task.ID, message)
	if len(idcode) < 3 {
		task.errc++
		fmt.Fprintf(task, "-error: the header(%v) is not correct on message:\n %v\n", parts[1], message)
		return
	}
	sid := fmt.Sprintf("%v-%v", idcode[0], idcode[1])
	host := task.Subs[sid]
	if len(host) < 1 {
		task.errc++
		fmt.Fprintf(task, "-error: host not found on message:\n %v\n", message)
		return
	}
	if idcode[2] == "0" {
		delete(task.Subs, sid)
		fmt.Fprintf(task, "%v->done well: %v\n", host, back)
	} else {
		task.errc++
		fmt.Fprintf(task, "%v->done error(%v): %v\n", host, idcode[2], back)
	}
}

func (t *Terminal) handleCallback() {
	for {
		select {
		case back := <-t.callback:
			message := string(back)
			if strings.Contains(message, "$") { //call back message
				break
			}
			t.handleMessage(message)
		case <-t.quit:
			return
		}
	}
}

func (t *Terminal) loopPing(name string, delay time.Duration) {
	log.Printf("Terminal ping to %v", name)
	data := "1234567890qwertyuiopasdfghjklzxcvbnm"
	t.running = true
	for t.running {
		used, call, back, err := t.C.PingSession(name, data)
		t.pslck.Lock()
		if err != nil {
			log.Printf("Terminal ping to %v fail with %v", name, err)
			t.pings[name] = "-1,-1,-1"
		} else {
			if used > 100 {
				log.Printf("Terminal ping to %v with %v,%v,%v", name, time.Duration(used)*time.Millisecond,
					time.Duration(call)*time.Millisecond, time.Duration(back)*time.Millisecond)
			}
			t.pings[name] = fmt.Sprintf("%v,%v,%v", time.Duration(used)*time.Millisecond,
				time.Duration(call)*time.Millisecond, time.Duration(back)*time.Millisecond)
		}
		t.pslck.Unlock()
		time.Sleep(delay)
	}
}

func (t *Terminal) NotifyTitle() {
	pings := []string{}
	t.pslck.RLock()
	for name, ps := range t.pings {
		pings = append(pings, fmt.Sprintf("%v(%v)", name, ps))
	}
	t.pslck.RUnlock()
	fmt.Fprintf(os.Stdout, "\033]0;%v(%v),%v\a", t.Name, t.ss.Len(), strings.Join(pings, ","))
}

func (t *Terminal) Activate(shell Shell) {
	fmt.Println()
	if t.activited == shell {
		fmt.Printf("%v is activated now", t.activited)
		t.activited.Write([]byte("\n"))
		return
	}
	if t.activited != nil {
		t.activited.Remove(t.stdout)
	}
	shell.Add(os.Stdout)
	_, err := shell.Write([]byte("\n"))
	if err != nil {
		shell.Remove(t.stdout)
		if t.activited != nil {
			t.activited.Add(t.stdout)
			fmt.Printf("%v activate fail with %v", shell, err)
			t.activited.Write([]byte("\n"))
		}
		return
	}
	t.last = shell.String()
	t.activited = shell
	fmt.Printf("%v is activated now(fast tap esc to quit)", t.activited)
	if t.activited == t.Cmd {
		t.NotifyTitle()
	}
}

func (t *Terminal) Switch(name string) (switched bool) {
	if name == t.Cmd.Name {
		switched = true
		t.Activate(t.Cmd)
		return
	}
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		if session.Name == name {
			switched = true
			t.Activate(session)
			return
		}
	}
	fmt.Printf("\nsession %v not found", name)
	t.activited.Write([]byte("\n"))
	return
}

func (t *Terminal) IdxSwitch(idx int) (switched bool) {
	if idx == 0 {
		t.Activate(t.Cmd)
		switched = true
		return
	}
	em := t.ss.Front()
	for i := 1; i < idx; i++ {
		if em != nil {
			em = em.Next()
		}
	}
	if em == nil {
		fmt.Printf("\nsession idx(%v) not found", idx)
		t.activited.Write([]byte("\n"))
		return
	}
	t.Activate(em.Value.(*SshSession))
	switched = true
	return
}

func (t *Terminal) AddSession(name, uri string, connect bool, env map[string]interface{}) (err error) {
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		if session.Name == name {
			err = fmt.Errorf("session %v already exists", name)
			return
		}
	}
	host, err := ParseSshHost(name, uri, env)
	if err != nil {
		return
	}
	fmt.Printf("add session by name(%v),channel(%v),host(%v),username(%v),password(%v)\n",
		host.Name, host.Channel, host.URI, host.Username, host.Password)
	session := NewSshSession(t.C, host)
	session.PreEnv = t.Env
	session.EnableCallback([]byte(t.CmdPrefix), t.callback)
	session.Add(NewNamedWriter(name, t.Log))
	if connect {
		err = session.Start()
		if err != nil {
			return
		}
		go session.Wait()
	}
	t.ss.PushBack(session)
	t.pslck.Lock()
	_, found := t.pings[host.Channel]
	if !found {
		t.pings[host.Channel] = "-1,-1,-1"
		go t.loopPing(host.Channel, 2*time.Second)
	}
	t.pslck.Unlock()
	return
}

func (t *Terminal) AddForward(m *fsck.Mapping) (err error) {
	fmt.Printf("add forward by name(%v),local(%v),remote(%v)\n", m.Name, m.Local, m.Remote)
	_, err = t.Forward.Start(m)
	return
}

func (t *Terminal) SaveConf() {
	if len(t.InstancePath) < 1 {
		return
	}
	conf := []map[string]interface{}{}
	data, err := ioutil.ReadFile(t.InstancePath)
	if err == nil {
		json.Unmarshal(data, &conf)
	}
	pwd, _ := os.Getwd()
	var name string
	if len(t.Name) > 0 && t.Name != "Sctrl" {
		name = t.Name
	} else {
		_, name = filepath.Split(pwd)
	}
	newone := map[string]interface{}{
		"web_url": t.WebSrv.URL,
		"pwd":     pwd,
		"name":    name,
		"last":    util.Now(),
	}
	for idx, cf := range conf {
		cpwd, _ := cf["pwd"].(string)
		if cpwd == pwd {
			conf[idx] = newone
			newone = nil
		}
	}
	if newone != nil {
		conf = append(conf, newone)
	}
	data, err = json.Marshal(conf)
	if err != nil {
		log.Printf("save instance info to %v fail with %v", t.InstancePath, err)
		return
	}
	os.Remove(t.InstancePath)
	err = ioutil.WriteFile(t.InstancePath, data, 0x775)
	if err != nil {
		log.Printf("save instance info to %v fail with %v", t.InstancePath, err)
		return
	}
	//log.Printf("save instance info to %v success", t.InstancePath)
}

func (t *Terminal) AllHostName() (ns []string) {
	for em := t.ss.Front(); em != nil; em = em.Next() {
		ns = append(ns, em.Value.(*SshSession).Name)
	}
	return
}

func (t *Terminal) Start(conf *WorkConf) (err error) {
	//initial
	err = t.WebSrv.Start()
	if err != nil {
		log.Printf("start web server fail with %v", err)
		return
	}
	log.Printf("listen web on %v", t.WebSrv.URL)
	t.Cmd.Env = append(t.Cmd.Env, t.Env...)
	t.Cmd.AddEnvf("%v=%v", KeyWebCmdURL, t.WebSrv.URL)
	t.Cmd.AddEnvf("HISTFILE=/tmp/.sctrl_%v_history", t.Name)
	t.Cmd.EnableCallback([]byte(t.CmdPrefix), t.callback)
	t.Cmd.Add(NewNamedWriter(t.Cmd.Name, t.Log))
	err = t.Cmd.Start()
	if err != nil {
		log.Printf("start sctrl command fail with %v", err)
		return
	}

	//
	go t.handleCallback()
	//
	for _, host := range conf.Hosts {
		if len(host.Name) < 1 || len(host.URI) < 1 {
			fmt.Printf("host conf %v is not correct,name/uri must be setted\n", MarshalAll(host))
			continue
		}
		err := t.AddSession(host.Name, host.URI, host.Startup > 0, host.Env)
		if err != nil {
			fmt.Printf("add session fail with %v\n", err)
		}
	}
	for _, forward := range conf.Forward {
		if len(forward.Name) < 1 || len(forward.Remote) < 1 {
			fmt.Printf("forward conf %v is not correct,name/remote must be setted\n", MarshalAll(forward))
			continue
		}
		err := t.AddForward(forward)
		if err != nil {
			fmt.Printf("add forward fail with %v\n", err)
		}
	}
	//
	t.IdxSwitch(0)
	//
	t.running = true
	go func() {
		for t.running {
			t.SaveConf()
			if t.activited == t.Cmd {
				t.NotifyTitle()
			}
			time.Sleep(5 * time.Second)
		}
	}()
	//
	go func() {
		var key []byte
		var escc int
		var ctrc int
		var sw []byte
		for t.running {
			select {
			case key = <-t.keyin:
			case <-t.ctrlc:
				continue
			}
			if bytes.Equal(key, CharTerm) {
				ctrc++
				if ctrc > 3 {
					t.CloseExit()
					t.keydone <- 1
					continue
				}
			} else {
				ctrc = 0
			}
			if bytes.Equal(key, CharESC) {
				escc++
				if escc > 2 {
					t.CloseExit()
					t.keydone <- 1
					continue
				}
			} else {
				escc = 0
			}
			if sw != nil {
				if bytes.Equal(key, CharEnter) {
					if len(sw) < 1 {
						fmt.Printf("\n\nAll Hosts:\n")
						WriteColumn(os.Stdout, t.AllHostName()...)
						fmt.Printf("\n\nPlease entry host name:")
					} else {
						t.Switch(string(sw))
						sw = nil
					}
				} else {
					if key[0] == 127 { //delete
						if len(sw) > 0 {
							sw = sw[0 : len(sw)-1]
							fmt.Printf("\b \b")
						} //else ignore
					} else {
						fmt.Printf("%v", string(key))
						sw = append(sw, key...)
					}
				}
				t.keydone <- 1
				continue
			}
			if bytes.Equal(key, CharCtrlb) {
				sw = []byte{}
				fmt.Printf("\n\nAll Hosts:\n")
				WriteColumn(os.Stdout, t.AllHostName()...)
				fmt.Printf("\n\nPlease entry host name:")
				t.keydone <- 1
				continue
			}
			switch {
			case bytes.Equal(key, KeyF1):
				t.IdxSwitch(0)
			case bytes.Equal(key, KeyF2):
				t.IdxSwitch(1)
			case bytes.Equal(key, KeyF3):
				t.IdxSwitch(2)
			case bytes.Equal(key, KeyF4):
				t.IdxSwitch(3)
			case bytes.Equal(key, KeyF5):
				t.IdxSwitch(4)
			case bytes.Equal(key, KeyF6):
				t.IdxSwitch(5)
			case bytes.Equal(key, KeyF7):
				t.IdxSwitch(6)
			case bytes.Equal(key, KeyF8):
				t.IdxSwitch(7)
			case bytes.Equal(key, KeyF9):
				t.IdxSwitch(8)
			case bytes.Equal(key, KeyF10):
				t.IdxSwitch(9)
			default:
				_, err = t.activited.Write(key)
				if err != nil {
					fmt.Printf("%v session fail with %v\n", t.activited, err)
				}
			}
			t.keydone <- 1
		}
	}()
	fmt.Println("sctrl start success...")
	return
}

func (t *Terminal) ProcReadkey() {
	//wait for cosole ready.
	time.Sleep(500 * time.Millisecond)
	signal.Notify(t.ctrlc, os.Interrupt)
	readkeyOpen("cli")
	for t.running {
		key, err := readkeyRead("cli")
		if err != nil {
			break
		}
		t.keyin <- key
		select {
		case <-t.keydone:
		case <-time.After(30 * time.Second):
			fmt.Printf("%v operationg timeout\n", t.activited)
			t.CloseExit()
		}
	}
}

func (t *Terminal) Close() (err error) {
	if t.closed {
		return
	}
	t.closed = true
	fmt.Printf("clean all...\n")
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		session.Close()
	}
	t.ss = list.New()
	fmt.Printf("closing all channel...\n")
	t.C.Close()
	fmt.Printf("closing sctrl console...\n")
	t.Cmd.Close()
	fmt.Printf("closing local log server...\n")
	t.Log.Close()
	fmt.Printf("closing local web server...\n")
	t.WebSrv.Close()
	fmt.Printf("closing forward channel server...\n")
	t.Forward.Close()
	readkeyClose("cli")
	t.running = false
	fmt.Printf("clean done...\n")
	return
}

func (t *Terminal) CloseExit() {
	fmt.Println()
	t.Close()
	exitf(0)
}
