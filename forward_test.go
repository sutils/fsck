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
	err := slaver.StartSlaver("localhost:9372", "master", "abc")
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
	testmap := func(name, local, remote string) error {
		m := &Mapping{
			Name:   name,
			Local:  local,
			Remote: remote,
		}
		_, err = forward.Start(m)
		if err != nil {
			return err
		}
		//
		conn, err := net.Dial("tcp", "localhost"+m.Local)
		if err != nil {
			return err
		}
		fmt.Fprintf(conn, "m2")
		buf := make([]byte, 1024)
		readed, err := conn.Read(buf)
		if err != nil {
			return err
		}
		if string(buf[:readed]) != "m2" {
			fmt.Println(buf[:readed])
			return fmt.Errorf("reply error")
		}
		return nil
	}
	//test spicial port
	err = testmap("x1", ":7234", "master://localhost:9392")
	if err != nil {
		t.Error(err)
		return
	}
	//test auto create port
	err = testmap("x2", "", "localhost:9392")
	if err != nil {
		t.Error(err)
		return
	}
	//test remote not found
	err = testmap("x3", ":7235", "not://localhost:9392")
	if err == nil {
		t.Error(err)
		return
	}
	//test error
	{
		//name repeat
		_, err = forward.Start(&Mapping{
			Name:   "x1",
			Local:  ":7234",
			Remote: "x://localhost:9392",
		})
		if err == nil {
			t.Error("nil")
			return
		}
		//url error
		_, err = forward.Start(&Mapping{
			Name:   "xmmm",
			Local:  "",
			Remote: "x://xx.com/s%CX%XX",
		})
		if err == nil {
			t.Error("nil")
			return
		}
		//local repeat
		_, err = forward.Start(&Mapping{
			Name:   "xx",
			Local:  ":7234",
			Remote: "x://localhost:9392",
		})
		if err == nil {
			t.Error("nil")
			return
		}
		//listen error
		_, err = forward.Start(&Mapping{
			Name:   "xx",
			Local:  ":7",
			Remote: "x://localhost:9392",
		})
		if err == nil {
			t.Error("nil")
			return
		}
	}
	ms := forward.List()
	if len(ms) < 2 {
		t.Error("mapping error")
		return
	}
	forward.Stop("x1", true)
	forward.Stop("x2", true)
	forward.Stop("x3", true)
	//test error
	{
		err = forward.Stop("not", false)
		if err == nil {
			t.Error("nil")
			return
		}
	}
	time.Sleep(time.Second)
	client.Close()
	slaver.Close()
	master.Close()
	time.Sleep(time.Second)

}
