package main

import (
	"C"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/sutils/fsck"
)
import (
	"container/list"
	"encoding/json"
	"io/ioutil"
	"log"
	"path/filepath"

	"github.com/sutils/readkey"
)

const (
	KeyWebCmdURL = "_web_cmd_url"
)

var SpaceRegex = regexp.MustCompile("[ ]+")
var CharTerm = []byte{3}
var CharESC = []byte{27}

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
	ID     string
	Subs   map[string]string
	reader io.ReadCloser
	writer io.WriteCloser
	errc   int
}

func NewTask() *Task {
	r, w := io.Pipe()
	return &Task{
		Subs:   map[string]string{},
		reader: r,
		writer: w,
	}
}

func (t *Task) Read(p []byte) (n int, err error) {
	return t.reader.Read(p)
}

func (t *Task) Write(p []byte) (n int, err error) {
	return t.writer.Write(p)
}

func (t *Task) Close() error {
	t.reader.Close()
	t.writer.Close()
	return nil
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
	WebSrv       *httptest.Server
	WebCmd       string //the web cmd path
	CmdPrefix    string
	InstancePath string
	Name         string
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
}

func NewTerminal(c *fsck.Slaver, ps1, shell, webcmd string) *Terminal {
	term := &Terminal{
		// ss:        map[string]*SshSession{},
		ss:           list.New(),
		slck:         sync.RWMutex{},
		C:            c,
		Mux:          http.NewServeMux(),
		Cmd:          NewCmd("sctrl", ps1, shell),
		Web:          NewWeb(nil),
		Log:          NewWebLogger(),
		WebCmd:       webcmd,
		CmdPrefix:    "-sctrl: ",
		callback:     make(chan []byte, 100),
		tasks:        map[string]*Task{},
		taskLck:      sync.RWMutex{},
		InstancePath: "/tmp/.sctrl_instance.json",
		Name:         "Sctrl",
	}
	term.Web.H = term.OnWebCmd
	term.WebSrv = httptest.NewUnstartedServer(term.Mux)
	term.Mux.Handle("/exec", term.Web)
	term.Mux.Handle("/log", term.Log)
	prefix := bytes.NewBuffer(nil)
	fmt.Fprintf(prefix, "set +o history\n")
	fmt.Fprintf(prefix, "alias sctrl='%v'\n", webcmd)
	fmt.Fprintf(prefix, "alias sadd='%v -run sadd'\n", webcmd)
	fmt.Fprintf(prefix, "alias srm='%v -run srm'\n", webcmd)
	fmt.Fprintf(prefix, "alias sall='%v -run sall'\n", webcmd)
	fmt.Fprintf(prefix, "alias spick='%v -run spick'\n", webcmd)
	fmt.Fprintf(prefix, "alias shelp='%v -run shelp'\n", webcmd)
	fmt.Fprintf(prefix, "alias sexec='%v -run sexec'\n", webcmd)
	fmt.Fprintf(prefix, "alias seval='%v -run seval'\n", webcmd)
	fmt.Fprintf(prefix, "history -d `history 1`\n")
	fmt.Fprintf(prefix, "set -o history\n")
	term.Cmd.Prefix = prefix
	return term
}

func (t *Terminal) OnWebCmd(w *Web, line string) (data interface{}, err error) {
	line = strings.TrimSpace(line)
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
			err = fmt.Errorf("Usage: sadd <name> <uri> [connect]")
			return
		}
		args := SpaceRegex.Split(cmds[1], 3)
		if len(args) < 2 {
			err = fmt.Errorf("Usage: sadd <name> <uri> [connect]")
			return
		}
		err = t.AddSession(args[0], args[1], len(args) > 2 && args[2] == "connect")
		if err == nil {
			data = fmt.Sprintf("sadd %v success\n", args[0])
		} else {
			err = fmt.Errorf("sadd %v fail with %v", args[0], err)
		}
		t.NotifyTitle()
		return
	case "srm":
		if len(cmds) < 2 {
			err = fmt.Errorf("Usage: srm <name>")
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
			err = fmt.Errorf("Usage: spick <name1> <name2> <...>")
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
		bys := buf.Bytes()
		if len(bys) > 0 {
			data = bys
		} else {
			data = []byte("ok")
		}
		return
	case "sexec":
		if len(cmds) < 2 {
			err = fmt.Errorf("Usage: sexec <cmd> <arg1> <arg2> <...>")
			return
		}
		task := NewTask()
		data = task
		go t.remoteExecf(task, "%v\n_sexec_code=$?\n", cmds[1])
		return
	case "seval":
		if len(cmds) < 2 {
			err = fmt.Errorf("Usage: seval <script file> <arg1> <arg2> <...>")
			return
		}
		args := strings.TrimSpace(cmds[1])
		var scriptPath, scriptArgs string
		if strings.HasPrefix(args, "'") {
			parts := strings.SplitN(args, "'", 2)
			if len(parts) < 2 {
				err = fmt.Errorf("%v having bash '", args)
				return
			}
			scriptPath = strings.TrimPrefix(parts[0], "'")
			scriptArgs = strings.TrimSpace(parts[1])
		} else if strings.HasPrefix(args, "\"") {
			parts := strings.SplitN(args, "\"", 2)
			if len(parts) < 2 {
				err = fmt.Errorf("%v having bash \"", args)
				return
			}
			scriptPath = strings.TrimPrefix(parts[0], "\"")
			scriptArgs = strings.TrimSpace(parts[1])
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
		var shell = bytes.NewBuffer(nil)
		fmt.Fprintf(shell, "cat >/tmp/sctrl-%v.sh <<EOF\n", "$_sexec_sid")
		shell.Write(scriptBytes)
		fmt.Fprintf(shell, "\nEOF\n")
		fmt.Fprintf(shell, "bash -e /tmp/sctrl-%v.sh %v\n", "$_sexec_sid", scriptArgs)
		fmt.Fprintf(shell, "_sexec_code=$?\n")
		fmt.Fprintf(shell, "rm -f /tmp/sctrl-%v.sh\n", "$_sexec_sid")
		task := NewTask()
		data = task
		go t.remoteExecf(task, string(shell.Bytes()))
		return
	case "shelp":
		fallthrough
	case "":
		buf := bytes.NewBuffer(nil)
		fmt.Fprintf(buf,
			` sadd <name> <uri> [connect]
	add session
 srm <name>
	remove session
 sall
	show all session
 spick <name1> <name2> <...>
	pick session, use 'spick all' to  pick all
 shelp
	show this
 sexec <cmd> <arg1> <arg2> <...>
	execute command on session
 seval <script file> <arg1> <arg2> <...>
    execute local script file to session.

`)
		data = buf.Bytes()
		return
	default:
		err = fmt.Errorf("-error: command %v not found", line)
	}
	return
}

func (t *Terminal) remoteExecf(task *Task, format string, args ...interface{}) {
	cmds := fmt.Sprintf(format, args...)
	log.Printf("remote execute->\n%v", cmds)
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
	t.tidc++
	task.ID = fmt.Sprintf("t%v", t.tidc)
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		name := session.Name
		if len(t.selected) < 1 || Having(t.selected, name) {
			sid := fmt.Sprintf("%v-%v", task.ID, name)
			//log.Printf("(%v) || echo '%v%v-$?'\n", cmds, t.CmdPrefix, sid)
			_, err := fmt.Fprintf(session, "_sexec_sid=%v\n%v\necho \"%v%v-$_sexec_code\"\n", sid, cmds, t.CmdPrefix, sid)
			if err == nil {
				task.Subs[sid] = name
				fmt.Fprintf(task, "%v->start success\n", name)
			} else {
				fmt.Fprintf(task, "%v->start fail with %v\n", name, err)
			}
		}
	}
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
	delete(task.Subs, sid)
	if idcode[2] == "0" {
		fmt.Fprintf(task, "%v->done well: %v\n", host, back)
	} else {
		fmt.Fprintf(task, "%v->done error(%v): %v\n", host, idcode[2], back)
	}
}

func (t *Terminal) handleCallback() {
	for {
		select {
		case back := <-t.callback:
			message := string(back)
			if strings.Contains(message, "-$") { //call back message
				break
			}
			t.handleMessage(message)
		case <-t.quit:
			return
		}
	}
}

func (t *Terminal) NotifyTitle() {
	fmt.Fprintf(os.Stdout, "\033]0;%v %v session\a", t.Name, t.ss.Len())
}

func (t *Terminal) Activate(shell Shell) {
	fmt.Println()
	if t.activited == shell {
		fmt.Printf("%v is activated now", t.activited)
		t.activited.Write([]byte("\n"))
		return
	}
	if t.activited != nil {
		t.activited.Remove(os.Stdout)
	}
	shell.Add(os.Stdout)
	_, err := shell.Write([]byte("\n"))
	if err != nil {
		shell.Remove(os.Stdout)
		if t.activited != nil {
			t.activited.Add(os.Stdout)
			fmt.Printf("%v activate fail with %v", shell, err)
			t.activited.Write([]byte("\n"))
		}
		return
	}
	t.last = shell.String()
	t.activited = shell
	fmt.Printf("%v is activated now", t.activited)
}

func (t *Terminal) Switch(name string) (switched bool) {
	if len(name) > 0 {
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
	if t.activited != t.Cmd {
		switched = true
		t.Activate(t.Cmd)
		return
	}
	if len(t.last) > 0 {
		for em := t.ss.Front(); em != nil; em = em.Next() {
			session := em.Value.(*SshSession)
			if session.Name == t.last {
				switched = true
				t.Activate(session)
				return
			}
		}
	}
	em := t.ss.Front()
	if em == nil {
		fmt.Print("\nnot running session found")
		t.activited.Write([]byte("\n"))
		return
	}
	session := em.Value.(*SshSession)
	switched = true
	t.Activate(session)
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
		if em == nil {
			break
		}
		em = em.Next()
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

func (t *Terminal) AddSession(name, uri string, connect bool) (err error) {
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		if session.Name == name {
			err = fmt.Errorf("session %v already exists", name)
			return
		}
	}
	host, err := ParseSshHost(name, uri)
	if err != nil {
		return
	}
	fmt.Printf("add session by name(%v),channel(%v),host(%v),username(%v),password(%v)\n",
		host.Name, host.Channel, host.URI, host.Username, host.Password)
	session := NewSshSession(t.C, host)
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
	log.Printf("save instance info to %v success", t.InstancePath)
}

func (t *Terminal) Proc(hosts ...*Host) (err error) {
	//initial
	t.WebSrv.Start()
	log.Printf("listen web on %v", t.WebSrv.URL)
	t.Cmd.AddEnvf("%v=%v", KeyWebCmdURL, t.WebSrv.URL)
	t.Cmd.EnableCallback([]byte(t.CmdPrefix), t.callback)
	t.Cmd.Add(NewNamedWriter(t.Cmd.Name, t.Log))
	err = t.Cmd.Start()
	if err != nil {
		return
	}
	t.SaveConf()
	//
	t.NotifyTitle()
	//
	go t.handleCallback()
	//
	for _, host := range hosts {
		if len(host.Name) < 1 || len(host.URI) < 1 {
			fmt.Printf("host conf %v is not correct,name/uri must be setted\n", MarshalAll(host))
			continue
		}
		err := t.AddSession(host.Name, host.URI, host.Startup > 0)
		if err != nil {
			fmt.Printf("add session fail with %v\n", err)
		}
	}
	//
	//
	t.Switch(t.Cmd.Name)
	//
	var key []byte
	t.running = true
	for t.running {
		key, err = readkey.ReadKey()
		if err != nil {
			t.CloseExit()
			break
		}
		switch {
		case bytes.Equal(key, CharTerm):
			t.CloseExit()
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
	}
	return
}

func (t *Terminal) Close() (err error) {
	fmt.Printf("clean all...\n")
	for em := t.ss.Front(); em != nil; em = em.Next() {
		session := em.Value.(*SshSession)
		session.Close()
	}
	t.ss = list.New()
	t.C.Close()
	readkey.Close()
	t.running = false
	fmt.Printf("clean done...\n")
	return
}

func (t *Terminal) CloseExit() {
	fmt.Println()
	t.Close()
	os.Exit(0)
}