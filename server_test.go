package fsck

import (
	"fmt"
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
		go func(c net.Conn) {
			buf := make([]byte, 1024)
			readed, err := c.Read(buf)
			if err != nil {
				panic(err)
			}
			fmt.Printf("read=>%v\n", buf[:readed])
			_, err = c.Write(buf[:readed])
			if err != nil {
				panic(err)
			}
			fmt.Printf("send=>%v\n", buf[:readed])
		}(con)
	}
}

// type buffer struct {
// 	r     io.ReadCloser
// 	W     io.Writer
// 	Reply *bytes.Buffer
// }

// func Newbuffer() *buffer {
// 	r, w := io.Pipe()
// 	return &buffer{
// 		r:     r,
// 		W:     w,
// 		Reply: bytes.NewBuffer(nil),
// 	}
// }

// func (b *buffer) Write(p []byte) (n int, err error) {
// 	return b.Reply.Write(p)
// }

// func (b *buffer) Read(p []byte) (n int, err error) {
// 	return b.r.Read(p)
// }

// func (b *buffer) Close() error {
// 	return b.r.Close()
// }

func TestRc(t *testing.T) {
	// netw.ShowLog = true
	// impl.ShowLog = true
	go runEchoServer()
	master := NewMaster()
	go master.Run(":9372", map[string]int{"abc": 1})
	time.Sleep(time.Second)
	slaver := NewSlaver("abc1")
	err := slaver.StartSlaver("localhost:9372", "x", "abc")
	if err != nil {
		t.Error("error")
		return
	}
	client := NewSlaver("abc2")
	err = client.StartClient("localhost:9372", "xxxx", "abc")
	if err != nil {
		t.Error("error")
		return
	}
	time.Sleep(time.Second)
	//
	session, err := client.DialSession("x", "localhost:9392")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Fprintf(session, "m1")
	buf := make([]byte, 1024)
	readed, err := session.Read(buf)
	if err != nil {
		t.Error(err)
		return
	}
	if string(buf[:readed]) != "m1" {
		t.Error("error")
		return
	}
	time.Sleep(2 * time.Second)
}
