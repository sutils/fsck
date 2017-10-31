package fsck

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func runEchoServer() {
	listen, err := net.Listen("tcp", ":9392")
	if err != nil {
		panic(err)
	}
	for {
		con, err := listen.Accept()
		if err != nil {
			panic(err)
		}
		go io.Copy(con, con)
	}
}

type buffer struct {
	r     io.ReadCloser
	W     io.Writer
	Reply *bytes.Buffer
}

func Newbuffer() *buffer {
	r, w := io.Pipe()
	return &buffer{
		r:     r,
		W:     w,
		Reply: bytes.NewBuffer(nil),
	}
}

func (b *buffer) Write(p []byte) (n int, err error) {
	return b.Reply.Write(p)
}

func (b *buffer) Read(p []byte) (n int, err error) {
	return b.r.Read(p)
}

func (b *buffer) Close() error {
	return b.r.Close()
}

func TestRc(t *testing.T) {
	// netw.ShowLog = true
	// impl.ShowLog = true
	go runEchoServer()
	master := NewMaster()
	go master.Run(":9372", map[string]int{"abc": 1})
	time.Sleep(time.Second)
	slaver := NewSlaver("abc1")
	err := slaver.Start("localhost:9372", "x", "abc", TypeSlaver)
	if err != nil {
		t.Error("error")
		return
	}
	client := NewSlaver("abc2")
	err = client.Start("localhost:9372", "x", "abc", TypeClient)
	if err != nil {
		t.Error("error")
		return
	}
	time.Sleep(time.Second)
	//
	buf := Newbuffer()
	_, err = client.Bind(buf, "x", "localhost:9392")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Fprintf(buf.W, "m1")
	for {
		reply := string(buf.Reply.Bytes())
		if reply != "m1" {
			fmt.Println("receive->", reply)
			time.Sleep(time.Second)
			continue
		} else {
			break
		}
	}
	time.Sleep(2 * time.Second)
}
