package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/kr/pty"
)

type Cmd struct {
	*exec.Cmd
	*MultiWriter
	Name   string
	pipe   *os.File
	out    *OutWriter
	Prefix io.Reader
}

func NewCmd(name, shell string) (cmd *Cmd) {
	cmd = &Cmd{
		Name:        name,
		Cmd:         exec.Command(shell),
		out:         NewOutWriter(),
		MultiWriter: NewMultiWriter(),
	}
	cmd.MultiWriter.Add(cmd.out)
	cmd.Env = os.Environ()
	return
}

func (c *Cmd) AddEnv(env ...string) {
	c.Env = append(c.Env, env...)
}

func (c *Cmd) AddEnvf(format string, args ...interface{}) {
	c.Env = append(c.Env, fmt.Sprintf(format, args...))
}

func (c *Cmd) String() string {
	return c.Name
}

func (c *Cmd) Start() (err error) {
	c.Env = append(c.Env, fmt.Sprintf("PS1=%v> ", c.Name))
	c.pipe, err = pty.Start(c.Cmd)
	if err != nil {
		return
	}
	go io.Copy(c.MultiWriter, c.pipe)
	if c.Prefix != nil {
		io.Copy(c, c.Prefix)
	}
	time.Sleep(100 * time.Millisecond)
	return
}

func (c *Cmd) EnableCallback(prefix []byte, back chan []byte) {
	c.out.EnableCallback(prefix, back)
}

func (c *Cmd) DisableCallback() {
	c.out.DisableCallback()
}

func (c *Cmd) Write(p []byte) (n int, err error) {
	n, err = c.pipe.Write(p)
	return
}
