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
import "syscall"

var SpaceRegex = regexp.MustCompile("[ ]+")

var CharTerm = []byte{3}
var CharESC = []byte{27}

type Terminal struct {
	ss      map[string]*SshSession
	target  string
	last    string
	running bool
	C       *fsck.Client
}

func NewTerminal(c *fsck.Client, uri string) *Terminal {
	return &Terminal{
		ss:     map[string]*SshSession{},
		target: "ctrl",
		C:      c,
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

func (t *Terminal) Proc() (err error) {
	SyncHistory()
	var char []byte
	t.running = true
	go t.HandleSignal()
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
			cmds := SpaceRegex.Split(line, -1)
			switch cmds[0] {
			case "ls":
				for name, session := range t.ss {
					fmt.Printf("%v:%v\n", name, session.Running)
				}
			case "add":
				if len(cmds) < 3 {
					fmt.Printf("Usage: add <name> <uri> [connect]\n")
					break
				}
				err = t.AddSession(cmds[1], cmds[2], len(cmds) > 3 && cmds[3] == "connect")
				if err == nil {
					fmt.Printf("add %v success\n", cmds[1])
				} else {
					fmt.Printf("add %v fail with %v\n", cmds[1], err)
				}
			case "rm":
				if len(cmds) < 2 {
					fmt.Printf("Usage: rm <name>\n")
					break
				}
				if session, ok := t.ss[cmds[1]]; ok {
					session.Close()
					delete(t.ss, cmds[1])
					fmt.Printf("rm session %v success\n", cmds[1])
				} else {
					fmt.Printf("session %v not found\n", cmds[1])
				}
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
