package main

import (
	"C"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strings"

	"github.com/pkg/term"
	"github.com/sutils/fsck"
)
import (
	"os/exec"
	"syscall"
)

var SpaceRegex = regexp.MustCompile("[ ]+")

var CharTerm = []byte{3}
var CharESC = []byte{27}

type Terminal struct {
	ss      map[string]*SshSession
	target  string
	last    string
	running bool
	C       *fsck.Client
	//
	selected []string
	bash     *Bash
}

func NewTerminal(c *fsck.Client, uri string) *Terminal {
	return &Terminal{
		ss:     map[string]*SshSession{},
		target: "ctrl",
		C:      c,
		bash:   NewBash("bash"),
	}
}

func (t *Terminal) readChar() (char []byte, err error) {
	tty, err := term.Open("/dev/tty")
	if err == nil {
		term.RawMode(tty)
		var readed int
		buf := make([]byte, 8)
		readed, err = tty.Read(buf)
		if err == nil {
			char = buf[0:readed]
		}
		tty.Restore()
		tty.Close()
	}
	return
}

func (t *Terminal) toHost() {
	if len(t.last) > 0 {
		if session, ok := t.ss[t.last]; ok {
			t.target = t.last
			session.Write([]byte("\n"))
			session.SetOut(os.Stdout)
			return
		}
	}
	for name, session := range t.ss {
		t.target = name
		t.last = name
		session.Write([]byte("\n"))
		session.SetOut(os.Stdout)
		return
	}
	fmt.Println("not running session found")
}

func (t *Terminal) toCtrl() {
	t.target = "ctrl"
}

func (t *Terminal) onKey(key int) int {
	switch key {
	case 27: //for esc
		fmt.Println()
		t.toHost()
	case 3: //for ctrl+c
		// fmt.Println()
		// t.Close()
	}
	return 0
}

func (t *Terminal) AddSession(name, uri string, connect bool) (err error) {
	if _, ok := t.ss[name]; ok {
		err = fmt.Errorf("session %v already exists", name)
		return
	}
	host, err := ParseSshHost(name, uri)
	if err != nil {
		return
	}
	session := NewSshSession(t.C, host)
	if connect {
		err = session.Start()
		if err != nil {
			return
		}
		go session.Wait()
	}
	t.ss[session.Name] = session
	return
}

func (t *Terminal) completer(query, ctx string) (options []string) {
	if query == ctx { //for command
		for _, cmd := range []string{"sls", "sadd", "srm", "select", "pwd", "cd", "eval"} {
			if strings.HasPrefix(cmd, query) {
				options = append(options, cmd)
			}
		}
		return
	}
	switch {
	case strings.HasPrefix(ctx, "pwd"):
		fallthrough
	case strings.HasPrefix(ctx, "sls"):
		fallthrough
	case strings.HasPrefix(ctx, "sadd"):
		return
	case strings.HasPrefix(ctx, "srm"):
		having := SpaceRegex.Split(strings.TrimSpace(strings.TrimPrefix(ctx, "srm ")), -1)
		for name := range t.ss {
			if Having(having, name) {
				continue
			}
			if len(query) < 1 || strings.HasPrefix(name, query) {
				options = append(options, name)
			}
		}
		return
	case strings.HasPrefix(ctx, "select"):
		having := SpaceRegex.Split(strings.TrimSpace(strings.TrimPrefix(ctx, "select ")), -1)
		for name := range t.ss {
			if Having(having, name) {
				continue
			}
			if len(query) < 1 || strings.HasPrefix(name, query) {
				options = append(options, name)
			}
		}
		return
	case strings.HasPrefix(ctx, "cd"):
		options = DirCompleter(query, ctx)
		return
	default:
		options = FilenameCompleter(query, ctx)
		return
	}
}

func (t *Terminal) Proc() (err error) {
	SyncHistory()
	t.AddSession("loc.m", "root:sco@loc.m:22", true)
	t.toHost()
	t.target = "loc.m"
	for {
		char, err := t.readChar()
		if err != nil {
			t.CloseExit()
			break
		}
		if bytes.Equal(char, CharTerm) {
			t.CloseExit()
			break
		}
		if bytes.Equal(char, CharESC) {
			fmt.Println()
			t.toCtrl()
			continue
		}
		session := t.ss[t.target]
		if session == nil {
			fmt.Println("session not found by " + t.target)
			t.toCtrl()
			continue
		}
		_, err = session.Write(char)
		if err != nil {
			fmt.Printf("%v session fail with %v\n", session.Name, err)
		}
	}
	var char []byte
	t.running = true
	Completer = t.completer
	go t.HandleSignal()
	t.bash.Start()
	t.selected = []string{}
	for t.running {
		if t.target == "ctrl" {
			baseline, err := StringCallback("ctrl> ", t.onKey)
			if err == io.EOF {
				break
			}
			line := strings.TrimSpace(baseline)
			if len(line) < 1 {
				continue
			}
			StoreHistory(baseline)
			cmds := SpaceRegex.Split(line, 2)
			switch cmds[0] {
			case "sls":
				if len(t.selected) > 0 {
					for _, name := range t.selected {
						if session, ok := t.ss[name]; ok {
							fmt.Printf("%v:%v\n", name, session.Running)
						} else {
							fmt.Printf("%v:%v\n", name, "not exists")
						}
					}
				} else {
					for name, session := range t.ss {
						fmt.Printf("%v:%v\n", name, session.Running)
					}
				}
			case "sadd":
				if len(cmds) < 2 {
					fmt.Printf("Usage: sadd <name> <uri> [connect]\n")
					break
				}
				args := SpaceRegex.Split(cmds[1], 3)
				if len(args) < 2 {
					fmt.Printf("Usage: sadd <name> <uri> [connect]\n")
					break
				}
				err = t.AddSession(args[0], args[1], len(args) > 2 && args[2] == "connect")
				if err == nil {
					fmt.Printf("sadd %v success\n", args[0])
				} else {
					fmt.Printf("sadd %v fail with %v\n", args[0], err)
				}
			case "srm":
				if len(cmds) < 2 {
					fmt.Printf("Usage: srm <name>\n")
					break
				}
				if session, ok := t.ss[cmds[1]]; ok {
					session.Close()
					delete(t.ss, cmds[1])
					fmt.Printf("srm session %v success\n", cmds[1])
				} else {
					fmt.Printf("session %v not found\n", cmds[1])
				}
			case "select":
				if len(cmds) < 2 {
					fmt.Printf("Usage: select <name1> <name2> ...\n")
					break
				}
				args := SpaceRegex.Split(cmds[1], -1)
				if args[0] == "all" {
					t.selected = []string{}
					break
				}
				t.selected = []string{}
				for _, name := range args {
					if _, ok := t.ss[name]; ok {
						t.selected = append(t.selected, name)
					} else {
						fmt.Printf("session %v not found, skipped\n", name)
					}
				}
			// case "pwd":
			// 	pwd, _ := os.Getwd()
			// 	fmt.Printf("%v\n", pwd)
			case "cd":
				if len(cmds) < 2 {
					fmt.Printf("Usage: cd <path>\n")
					break
				}
				err := os.Chdir(cmds[1])
				if err != nil {
					fmt.Printf("%v\n", err)
				}
			// case "export":
			// 	if len(cmds) < 2 {
			// 		fmt.Printf("Usage: export <key=value>\n")
			// 		break
			// 	}
			// 	kvs := strings.SplitN(cmds[1], "=", 2)
			// 	t.envs[kvs[0]] = cmds[1]
			// case "eval":
			default:
				cmd := exec.Command("bash", "-c", fmt.Sprintf("source /tmp/env.sh >/dev/null 2>/dev/null && %v && set>/tmp/env.sh", line))
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Stdin = os.Stdin
				cmd.Run()
				// t.bash.Exec(line)
			}
		} else {
			char, err = t.readChar()
			if err != nil {
				t.CloseExit()
				break
			}
			if bytes.Equal(char, CharTerm) {
				t.CloseExit()
				break
			}
			if bytes.Equal(char, CharESC) {
				fmt.Println()
				t.toCtrl()
				continue
			}
			session := t.ss[t.target]
			if session == nil {
				fmt.Println("session not found by " + t.target)
				t.toCtrl()
				continue
			}
			_, err = session.Write(char)
			if err != nil {
				fmt.Printf("%v session fail with %v\n", session.Name, err)
			}
		}
	}
	Completer = EmptyCompleter
	return
}

func (t *Terminal) HandleSignal() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	signal.Notify(signals, os.Interrupt)
	for s := range signals {
		switch s {
		case syscall.SIGWINCH:
			ResizeTerminal()
		case os.Interrupt:
			t.CloseExit()
		}
	}
}

func (t *Terminal) Close() (err error) {
	fmt.Printf("clean all...\n")
	for _, session := range t.ss {
		session.Close()
	}
	t.C.Close()
	Cleanup()
	t.running = false
	fmt.Printf("clean done...\n")
	return
}

func (t *Terminal) CloseExit() {
	fmt.Println()
	t.Close()
	os.Exit(0)
}
