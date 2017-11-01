package fsck

import (
	"fmt"
	"net"
)

func init() {
	ShowLog = 1
	go runEchoServer()
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
