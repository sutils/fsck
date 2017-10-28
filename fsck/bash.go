package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Bash struct {
	C     *exec.Cmd
	Shell string
	// Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	stdin  io.Writer
	stdout io.Reader
	wait   chan string
	key    string
	eidc   uint32
	lck    sync.RWMutex
}

func NewBash(shell string) *Bash {
	return &Bash{
		C:      exec.Command(shell),
		Shell:  shell,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		wait:   make(chan string),
		lck:    sync.RWMutex{},
	}
}

func (b *Bash) Start() (err error) {
	b.C.Stderr = b.Stderr
	// if err != nil {
	// 	return
	// }
	b.stdout, err = b.C.StdoutPipe()
	// if err != nil {
	// 	return
	// }
	b.stdin, err = b.C.StdinPipe()
	// if err != nil {
	// 	return
	// }
	go b.readStdout()
	return b.C.Start()
}

func (b *Bash) readStdout() {
	reader := bufio.NewReader(b.stdout)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		// fmt.Println(line, strings.Contains(line, b.key))
		if len(b.key) > 0 && strings.Contains(line, b.key) {
			b.wait <- "done"
			line = strings.Replace(line, b.key, "", 1)
			if len(line) < 1 {
				continue
			}
		}
		fmt.Fprintf(b.Stdout, "%v", line)
	}
}

func (b *Bash) Exec(line string) {
	b.lck.Lock()
	defer b.lck.Unlock()
	line = strings.TrimSpace(line)
	if strings.HasSuffix(line, "&") {
		fmt.Fprintf(b.stdin, "%v\n", line)
		time.Sleep(100 * time.Millisecond)
		return
	}
	b.key = fmt.Sprintf("-a%va-\n", b.eidc)
	fmt.Fprintf(b.stdin, "%v && echo %v", line, b.key)
	<-b.wait
}
