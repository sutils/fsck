package fsck

import (
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

type testServer struct {
	net.Listener
}

func listenTestServer(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	server := &testServer{Listener: listener}
	server.run()
}

func (t *testServer) run() {
	for {
		con, err := t.Accept()
		if err != nil {
			break
		}
		go func() {
			for {
				buf := make([]byte, 10240)
				readed, err := con.Read(buf)
				if err != nil {
					break
				}
				// fmt.Println("->", string(buf[:readed]))
				con.Write(buf[:readed])
			}
		}()
	}
}

type testClient struct {
	*Channel
}

func newTestclient(uri string) *testClient {
	raw, err := net.Dial("tcp", uri)
	if err != nil {
		panic(err)
	}
	return &testClient{
		Channel: &Channel{
			Conn: raw,
		},
	}
}

func (t *testClient) exec() {
	_, err := t.Write([]byte("message"))
	if err != nil {
		panic(err)
	}
	buf := make([]byte, 7)
	err = t.ReadW(buf)
	if err != nil {
		panic(err)
	}
	if string(buf) != "message" {
		fmt.Println(buf)
		panic("data->" + string(buf))
	}
}

func TestFSck(t *testing.T) {
	go listenTestServer(":9723")
	server, err := NewServer(":9623", map[string]int{
		"abc": 1,
	})
	if err != nil {
		t.Error(err)
		return
	}
	go server.Run()
	forward := NewForward(NewClient("localhost:9623", "abc"))
	go forward.Run(":9523", "localhost:9723")
	wg := sync.WaitGroup{}
	for k := 0; k < 5; k++ {
		wg.Add(1)
		go func() {
			tc := newTestclient("localhost:9523")
			for i := 0; i < 10; i++ {
				tc.exec()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
