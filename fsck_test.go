package fsck

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func init() {
	ShowLog = 2
	echo, err := NewEchoServer("tcp", ":9392")
	if err != nil {
		panic(err)
	}
	echo.Start()
	// go runEchoServer()
}

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
				return
			}
			fmt.Printf("read=>%v\n", buf[:readed])
			_, err = c.Write(buf[:readed])
			if err != nil {
				return
			}
			fmt.Printf("send=>%v\n", buf[:readed])
		}(con)
	}
}

func TestPipeConn(t *testing.T) {
	a, b, err := CreatePipedConn()
	if err != nil {
		t.Error("error")
		return
	}
	go io.Copy(b, b)
	fmt.Fprintf(a, "val-%v", 0)
	buf := make([]byte, 100)
	readed, err := a.Read(buf)
	if err != nil {
		t.Error("error")
		return
	}
	fmt.Printf("-->%v\n", string(buf[0:readed]))
	a.Close()
	b.Close()
	//for cover
	a.SetDeadline(time.Now())
	a.SetReadDeadline(time.Now())
	a.SetWriteDeadline(time.Now())
	fmt.Println(a.LocalAddr(), a.RemoteAddr(), a.String(), a.Network())
}
