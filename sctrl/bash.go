package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/kr/pty"
	"github.com/sutils/readkey"
)

type Cmd struct {
	*exec.Cmd
	*MultiWriter
	Name   string
	PS1    string
	pipe   *os.File
	out    *OutWriter
	Prefix io.Reader
	OnExit func(err error)
	Resize bool
}

func NewCmd(name, ps1, shell string) (cmd *Cmd) {
	cmd = &Cmd{
		Name:        name,
		PS1:         ps1,
		Cmd:         exec.Command(shell),
		out:         NewOutWriter(),
		MultiWriter: NewMultiWriter(),
		Resize:      true,
	}
	cmd.MultiWriter.Add(cmd.out)
	cmd.Env = os.Environ()
	return
}

func (c *Cmd) AddEnvf(format string, args ...interface{}) {
	c.Env = append(c.Env, fmt.Sprintf(format, args...))
}

func (c *Cmd) String() string {
	return c.Name
}

func (c *Cmd) Start() (err error) {
	if len(c.PS1) > 0 {
		c.Env = append(c.Env, "PS1="+c.PS1)
	}
	//
	var vpty, tty *os.File
	vpty, tty, err = pty.Open()
	if err != nil {
		tty.Close()
		vpty.Close()
		return
	}
	if c.Resize {
		w, h := readkey.GetSize()
		err = readkey.SetSize(vpty.Fd(), w, h)
		if err != nil {
			tty.Close()
			vpty.Close()
			return
		}
	}
	c.Cmd.Stdout = tty
	c.Cmd.Stdin = tty
	c.Cmd.Stderr = tty
	if c.Cmd.SysProcAttr == nil {
		c.Cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.Cmd.SysProcAttr.Setctty = true
	c.Cmd.SysProcAttr.Setsid = true
	err = c.Cmd.Start()
	if err != nil {
		tty.Close()
		vpty.Close()
		return
	}
	tty.Close()
	c.pipe = vpty
	//
	go func() {
		_, err = io.Copy(c.MultiWriter, c.pipe)
		if c.OnExit != nil {
			c.OnExit(err)
		}
	}()
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

func (c *Cmd) Close() error {
	c.DisableCallback()
	return c.pipe.Close()
}
