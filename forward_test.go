package fsck

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestForwad(t *testing.T) {
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
	forward := NewForward(client)
	_, err = forward.Start(&Mapping{
		Name:   "x1",
		Local:  ":7234",
		Remote: "x://localhost:9392",
	})
	if err != nil {
		t.Error(err)
		return
	}
	conn, err := net.Dial("tcp", "localhost:7234")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Fprintf(conn, "m2")
	buf := make([]byte, 1024)
	readed, err := conn.Read(buf)
	if err != nil {
		t.Error(err)
		return
	}
	if string(buf[:readed]) != "m2" {
		fmt.Println(buf[:readed])
		t.Error("error")
	}
	forward.Stop("x1", true)
	time.Sleep(time.Second)
	client.Close()
	slaver.Close()
	master.Close()
	time.Sleep(time.Second)

}
