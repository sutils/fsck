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
	ss        *list.List
	slck      sync.RWMutex
	Cmd       *Cmd
	Web       *Web
	Log       *WebLogger
	last      string
	running   bool
	C         *fsck.Client
	Mux       *http.ServeMux
	WebSrv    *httptest.Server
	WebCmd    string //the web cmd path
	CmdPrefix string
	ConfPath  string
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

func NewTerminal(c *fsck.Client, uri, shell, webcmd string) *Terminal {
	term := &Terminal{
		// ss:        map[string]*SshSession{},
		ss:        list.New(),
		slck:      sync.RWMutex{},
		C:         c,
		Mux:       http.NewServeMux(),
		Cmd:       NewCmd("sctrl", shell),
		Web:       NewWeb(nil),
		Log:       NewWebLogger(),
		WebCmd:    webcmd,
		CmdPrefix: "-sctrl: ",
		callback:  make(chan []byte, 100),
		tasks:     map[string]*Task{},
		taskLck:   sync.RWMutex{},
		ConfPath:  "/tmp/fsck_conf.json",
	}
	term.Web.H = term.OnWebCmd
	term.WebSrv = httptest.NewUnstartedServer(term.Mux)
	term.Mux.Handle("/exec", term.Web)
	term.Mux.Handle("/log", term.Log)
	prefix := bytes.NewBuffer(nil)
	fmt.Fprintf(prefix, "set +o history\n")
	fmt.Fprintf(prefix, "alias sctrl='%v'\n", webcmd)
	fmt.Fprintf(prefix, "alias sadd='%v -sadd'\n", webcmd)
	fmt.Fprintf(prefix, "alias srm='%v -srm'\n", webcmd)
	fmt.Fprintf(prefix, "alias sall='%v -sall'\n", webcmd)
	fmt.Fprintf(prefix, "alias spick='%v -spick'\n", webcmd)
	fmt.Fprintf(prefix, "alias shelp='%v -shelp'\n", webcmd)
	fmt.Fprintf(prefix, "alias sexec='%v -sexec'\n", webcmd)
	fmt.Fprintf(prefix, "alias seval='%v -seval'\n", webcmd)
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
	fmt.Fprintf(os.Stdout, "\033]0;%v %v session\a", t.Cmd.Name, t.ss.Len())
}

func (t *Terminal) Activate(shell Shell) {
	if t.activited == shell {
		fmt.Printf("\n%v is activated now", t.activited)
		t.activited.Write([]byte("\n"))
		return
	}
	shell.Add(os.Stdout)
	_, err := shell.Write([]byte("\n"))
	if err != nil {
		fmt.Printf("%v activate fail with %v\n", shell, err)
		shell.Remove(os.Stdout)
		return
	}
	if t.activited != nil {
		t.activited.Remove(os.Stdout)
	}
	t.last = shell.String()
	fmt.Printf("\n%v is activated now", t.last)
	t.activited = shell
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
	if len(t.ConfPath) < 1 {
		return
	}
	conf := map[string]interface{}{
		"web_url": t.WebSrv.URL,
	}
	data, err := json.Marshal(conf)
	if err != nil {
		log.Printf("save conf to %v fail with %v", t.ConfPath, err)
		return
	}
	os.Remove(t.ConfPath)
	err = ioutil.WriteFile(t.ConfPath, data, 0x775)
	if err != nil {
		log.Printf("save conf to %v fail with %v", t.ConfPath, err)
		return
	}
}

func (t *Terminal) Proc() (err error) {
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
	//
	t.Switch(t.Cmd.Name)
	t.NotifyTitle()
	//
	go t.handleCallback()
	//
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
