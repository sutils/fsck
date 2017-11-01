package fsck

import (
	"fmt"
	"testing"
	"time"
)

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
	server := NewServer()
	go server.Run(":9372", map[string]int{"abc": 1})
	time.Sleep(time.Second)
	//
	client := NewSlaver("abc2")
	err := client.StartClient("localhost:9372", "xxxx", "abc")
	if err != nil {
		t.Error("error")
		return
	}
	time.Sleep(time.Second)
	//
	session, err := client.DialSession("master", "localhost:9392")
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
	time.Sleep(1 * time.Second)
	client.Close()
	server.Close()
	time.Sleep(1 * time.Second)
}
