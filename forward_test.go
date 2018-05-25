package fsck

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Centny/gwf/routing"
	"github.com/Centny/gwf/routing/httptest"
)

func TestForwad(t *testing.T) {
	master := NewMaster()
	master.SP.RegisterDefaulDialer()
	go master.Run(":9372", map[string]int{"abc": 1})
	time.Sleep(time.Second)
	slaver := NewSlaver("abc1")
	slaver.SP.RegisterDefaulDialer()
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
	forward := NewForward(client.DialSession)
	forward.WebSuffix = ".loc"
	//
	testmap := func(name, uri string) error {
		m, err := forward.AddUriForward(name, uri)
		if err != nil {
			return err
		}
		//
		conn, err := net.Dial("tcp", m.Local.Host)
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
	{ //test server forward
		forward2 := NewForward(master.DialSession)
		_, err = forward2.AddUriForward("xz-0", "tcp://:23211<master>tcp://localhost:9392")
		if err != nil {
			t.Error(err)
			return
		}
		con, err := net.Dial("tcp", "localhost:23211")
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(con, "value-%v", 1)
		buf := make([]byte, 100)
		readed, err := con.Read(buf)
		if err != nil {
			t.Error(err)
			return
		}
		if string(buf[0:readed]) != "value-1" {
			t.Error("error")
			return
		}
	}
	{ //test web forward
		_, err = forward.AddUriForward("wtest-0", "tcp://:2831<master>tcp://localhost:9392")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("wtest-1", "tcp://:2830<master>tcp://localhost:2834")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("wtest-2", "web://loctest0<master>http://web?dir=/tmp")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("wtest-3", "web://loctest2<master>http://127.0.0.1")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("wtest-4", "web://loctest1<master>https://www.kuxiao.cn")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("wtest-5", "web://loctest3<testx>http://192.168.1.1")
		if err != nil {
			t.Error(err)
			return
		}

		ts := httptest.NewServer(func(hs *routing.HTTPSession) routing.HResult {
			// if hs.R.URL.Path == "/web" {
			// 	return server.ListWebForward(hs)
			// }
			name := strings.TrimPrefix(hs.R.URL.Path, "/web/")
			switch name {
			case "loctest0":
				hs.R.Host = "loctest0.loc"
			case "loctest1":
				hs.R.Host = "loctest1.loc"
			case "loctest2":
				hs.R.Host = "loctest2.loc"
			case "loctest3":
				hs.R.Host = "loctest3.loc"
			case "loctest4":
				hs.R.Host = "loctest4.loc"
			}
			hs.R.URL.Path = "/"
			return forward.ProcWebForward(hs)
		})
		//
		// data, err := ts.G("/web")
		// if err != nil {
		// 	t.Errorf("%v-%v", err, data)
		// 	return
		// }
		// fmt.Printf("data->:\n%v\n\n\n\n", data)
		//
		data, err := ts.G("/web/loctest0")
		if err != nil {
			t.Errorf("%v-%v", err, data)
			return
		}
		fmt.Printf("data->:\n%v\n\n\n\n", data)
		//
		data, err = ts.G("/web/loctest1")
		if err != nil {
			t.Errorf("%v-%v", err, data)
			return
		}
		fmt.Printf("data->:\n%v\n\n\n\n", data)
		//
		data, err = ts.G("/web/loctest2")
		if err != nil {
			t.Errorf("%v-%v", err, data)
			return
		}
		fmt.Printf("data->:\n%v\n\n\n\n", data)
		//
		data, err = ts.G("/web/loctest3")
		if err == nil {
			t.Errorf("%v-%v", err, data)
			return
		}
		fmt.Printf("data->:\n%v\n\n\n\n", data)
		//
		data, err = ts.G("/web/loctest4")
		if err == nil {
			t.Errorf("%v-%v", err, data)
			return
		}
		fmt.Printf("data->:\n%v\n\n\n\n", data)
		//
		forward.WebAuth = "test:123"
		data, err = ts.G("/web/loctest3")
		if err == nil {
			t.Errorf("%v-%v", err, data)
			return
		}
		fmt.Printf("data->:\n%v\n\n\n\n", data)
		forward.WebAuth = ""
		//
		forward.RemoveForward("tcp://:2831")
	}
	{ //test tcp forward
		//test spicial port
		err = testmap("x1", "tcp://:7234<master>tcp://localhost:9392")
		if err != nil {
			t.Error(err)
			return
		}
		//test auto create port
		err = testmap("x2", "tcp://<master>tcp://localhost:9392")
		if err != nil {
			t.Error(err)
			return
		}
		//test remote not found
		err = testmap("x3", "tcp://:7235<not>tcp://localhost:9392")
		if err == nil {
			t.Error(err)
			return
		}
	}
	{ //test forward limit
		_, err = forward.AddUriForward("xv-0", "tcp://:23221?limit=1<master>tcp://localhost:9392")
		if err != nil {
			t.Error(err)
			return
		}
		con, err := net.Dial("tcp", "localhost:23221")
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(con, "value-%v", 1)
		buf := make([]byte, 100)
		readed, err := con.Read(buf)
		if err != nil {
			t.Error(err)
			return
		}
		if string(buf[0:readed]) != "value-1" {
			t.Error("error")
			return
		}
		_, err = net.Dial("tcp", "localhost:23221")
		if err == nil {
			t.Error(err)
			return
		}
		time.Sleep(200 * time.Millisecond)
		err = forward.RemoveForward("tcp://:23221")
		if err == nil {
			t.Error(err)
			return
		}
	}
	{ //add/remove forward
		_, err = forward.AddUriForward("xy-0", "tcp://:24221<master>tcp://localhost:2422")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("xy-1", "tcp://:24221<master>tcp://localhost:2422") //repeat
		if err == nil {
			t.Error(err)
			return
		}
		//
		_, err = forward.AddUriForward("xy-2", "tcp://:2322?limit=xxx<master>tcp://localhost:2422")
		if err != nil {
			t.Error(err)
			return
		}
		//
		_, err = forward.AddUriForward("xy-3", "web://loc1<master>http://localhost:2422")
		if err != nil {
			t.Error(err)
			return
		}
		_, err = forward.AddUriForward("xy-4", "web://loc1<master>http://localhost:2422") //repeat
		if err == nil {
			t.Error(err)
			return
		}
		//
		_, err = forward.AddUriForward("xy-5", "xxxx://:24221<master>tcp://localhost:2422") //not suppored
		if err == nil {
			t.Error(err)
			return
		}
		//
		// err = forward.RemoveForward("tcp://:24221")
		// if err != nil {
		// 	t.Error(err)
		// 	return
		// }
		err = forward.RemoveForward("tcp://:2322")
		if err != nil {
			t.Error(err)
			return
		}
		err = forward.RemoveForward("web://loc1")
		if err != nil {
			t.Error(err)
			return
		}
		//test error
		err = forward.RemoveForward("tcp://:283x")
		if err == nil {
			t.Error(err)
			return
		}
		err = forward.RemoveForward("web://loctestxxx")
		if err == nil {
			t.Error(err)
			return
		}
		err = forward.RemoveForward("://loctestxxx")
		if err == nil {
			t.Error(err)
			return
		}
	}
	{ //test forward name not found
		_, err = forward.AddUriForward("xc-0", "tcp://:23221<xxxx>tcp://localhost:9392")
		if err != nil {
			t.Error(err)
			return
		}
		con, err := net.Dial("tcp", "localhost:23221")
		if err != nil {
			t.Error(err)
			return
		}
		buf := make([]byte, 100)
		_, err = con.Read(buf)
		if err == nil {
			t.Error(err)
			return
		}
		err = forward.RemoveForward("tcp://:23221")
		if err != nil {
			t.Error(err)
			return
		}
	}
	//test error
	{
		//name repeat
		_, err = forward.AddUriForward("x1", "tcp://:7234<x>tcp://localhost:9392")
		if err == nil {
			t.Error("nil")
			return
		}
		//local repeat
		_, err = forward.AddUriForward("xx", "tcp://:7234<x>tcp://localhost:9392")
		if err == nil {
			t.Error("nil")
			return
		}
		//listen error
		_, err = forward.AddUriForward("xx", "tcp://:7<x>tcp://localhost:9392")
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
	// forward.Stop("x3", true)
	//test error
	{
		err = forward.Stop("not", false)
		if err == nil {
			t.Error("nil")
			return
		}
	}
	forward.Close()
	time.Sleep(time.Second)
	client.Close()
	slaver.Close()
	master.Close()
	time.Sleep(time.Second)

}

func TestMapping(t *testing.T) {
	mapping, err := NewMapping("abc", "tcp://:23i2?limit=1<test>tcp://localhost:80?arg1=abc")
	if err != nil {
		t.Error(err)
		return
	}
	var limit int
	err = mapping.LocalValidF(`limit,R|I,R:0`, &limit)
	if err != nil || limit != 1 {
		t.Errorf("%v,%v", err, limit)
		return
	}
	var arg1 string
	err = mapping.RemoteValidF(`arg1,R|S,L:0`, &arg1)
	if err != nil || limit != 1 {
		t.Errorf("%v,%v", err, limit)
		return
	}

	//
	_, err = NewMapping("xxx", "xx")
	if err == nil {
		t.Error(err)
		return
	}
}
