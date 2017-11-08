package fsck

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Centny/gwf/netw"
	"github.com/Centny/gwf/netw/rc"
	"github.com/Centny/gwf/pool"

	"github.com/Centny/gwf/util"
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
	server.HbDelay = 3000
	go server.Run(":9372", nil)
	time.Sleep(time.Second)
	server.L.AddToken(map[string]int{"abc": 2})
	//
	client := NewSlaver("abc2")
	client.HbDelay = 3000
	err := client.StartClient("localhost:9372", "xxxx", "abc")
	if err != nil {
		t.Error("error")
		return
	}
	time.Sleep(time.Second)
	// client.R.Login_(token string)
	//
	//test ping
	used, slaver, err := client.PingExec("master", "data")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("ping to master used ", used, slaver)
	//
	used, slaverCall, slaverBack, err := client.PingSession("master", "data")
	if err != nil {
		t.Error(err)
		return
	}
	fmt.Println("ping to master used ", used, slaverCall, slaverBack)
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
	{
		//test slaver login not name
		wait := make(chan error)
		slaver1 := NewSlaver("xxx1")
		slaver1.OnLogin = func(l *rc.AutoLoginH, err error) {
			wait <- err
		}
		slaver1.Start("localhost:9372", "", "", "abc", TypeSlaver)
		err = <-wait
		if err == nil {
			t.Error(err)
			return
		}
		fmt.Println("test slaver login not name done...")
		//test client login not session
		slaver2 := NewSlaver("xxx2")
		slaver2.OnLogin = func(l *rc.AutoLoginH, err error) {
			wait <- err
		}
		slaver2.Start("localhost:9372", "", "", "abc", TypeClient)
		err = <-wait
		if err == nil {
			t.Error(err)
			return
		}
		fmt.Println("test client login not name done..")
	}
	//test error
	{
		//test ping to unknow
		_, _, err = client.PingExec("xxxx", "data")
		if err == nil {
			t.Error(err)
			return
		}
		_, _, err = client.PingExec("", "data")
		if err == nil {
			t.Error(err)
			return
		}
		//dial to unknow remote
		_, err = client.DialSession("master", "localhost:732")
		if err == nil {
			t.Error(err)
			return
		}
		//dial to unknow name
		_, err = client.DialSession("xxxs", "localhost:732")
		if err == nil {
			t.Error(err)
			return
		}
		//dial url empty
		_, err = client.DialSession("uuux", "")
		if err == nil {
			t.Error(err)
			return
		}
		//cid is not found
		server.slavers["mock_s"] = "cxx"
		server.clients["mock_c"] = "cxx"
		_, err = client.DialSession("mock_s", "localhost:732")
		if err == nil {
			t.Error(err)
			return
		}
		//close unknow sid
		err = client.CloseSession(1000)
		if err == nil {
			t.Error(err)
			return
		}
		//mock remote session not found
		session, err := client.SP.Dail(100, "localhost:9392", ioutil.Discard)
		if err != nil {
			t.Error(err)
			return
		}
		server.si2n[fmt.Sprintf("xxxx-%v", session.SID)] = "master"
		err = client.CloseSession(session.SID)
		if err == nil {
			t.Error(err)
			return
		}
		//mock remote clinet not found
		session, err = client.SP.Dail(101, "localhost:9392", ioutil.Discard)
		if err != nil {
			t.Error(err)
			return
		}
		err = client.CloseSession(session.SID)
		if err == nil {
			t.Error(err)
			return
		}
		//login error
		err = client.R.LoginV("abc", nil)
		if err == nil {
			t.Error(err)
			return
		}
		err = client.R.LoginV("not", util.Map{})
		if err == nil {
			t.Error(err)
			return
		}
		err = client.R.LoginV("abc", util.Map{
			"ctype": "xxx",
		})
		if err == nil {
			t.Error(err)
			return
		}
		//
		//close sid not setted.
		_, err = client.R.Exec_m("/usr/close", nil)
		if err == nil {
			t.Error(err)
			return
		}
		//
		//dial to client, uri empty
		cid := server.slavers["master"]
		cmdc := server.L.CmdC(cid)
		_, err = cmdc.Exec_m("dial", util.Map{})
		if err == nil {
			t.Error(err)
			return
		}
		//close sid not setted
		_, err = cmdc.Exec_m("close", util.Map{})
		if err == nil {
			t.Error(err)
			return
		}
		//client data error
		_, err = client.Channel.Write([]byte{0})
		if err == nil {
			t.Error(err)
			return
		}
		_, err = client.Channel.Write([]byte{0, 0, 6})
		if err == nil {
			t.Error(err)
			return
		}
		server.si2n["xxxx-6"] = "cxx"
		_, err = client.Channel.Write([]byte{0, 0, 6})
		if err == nil {
			t.Error(err)
			return
		}
		server.si2n["xxxx-7"] = "master"
		_, err = client.Channel.Write([]byte{0, 0, 7})
		if err == nil {
			t.Error(err)
			return
		}
		//slaver data error
		_, err = server.Local.Channel.Write([]byte{0})
		if err == nil {
			t.Error(err)
			return
		}
		_, err = server.Local.Channel.Write([]byte{0, 0, 6})
		if err == nil {
			t.Error(err)
			return
		}
		server.ni2s["master-6"] = "cxx"
		_, err = server.Local.Channel.Write([]byte{0, 0, 6})
		if err == nil {
			t.Error(err)
			return
		}
		server.si2n["master-7"] = "xxxx"
		_, err = server.Local.Channel.Write([]byte{0, 0, 7})
		if err == nil {
			t.Error(err)
			return
		}
		//remote closed by sid
		session = server.Local.SP.Start(8, ioutil.Discard)
		server.si2n[fmt.Sprintf("xxxx-%v", session.SID)] = "master"
		session.Raw.Close()
		_, err = client.Channel.Write([]byte{0, 0, byte(8), 0, 0})
		if err != ErrSessionClosed {
			t.Error(err)
			return
		}
		//
		//session wirte error
		session = server.Local.SP.Start(9, &mockwriter{})
		session.Timeout, session.MaxDelay = 1*time.Second, 200*time.Millisecond
		_, err = session.Write([]byte("test"))
		if err != io.EOF {
			t.Error(err)
			return
		}
		_, err = session.Write([]byte("test"))
		if err != io.EOF {
			t.Error(err)
			return
		}
		_, err = session.Write([]byte("test"))
		if err != io.EOF {
			t.Error(err)
			return
		}
		//other error
		//
		_, err = client.Channel.SP.Write(nil)
		if err == nil {
			t.Error(err)
			return
		}
		//
		func() {
			defer func() {
				recover()
			}()
			(&Session{}).Read(nil)
		}()
		//
		server.Master.OnCmd(nil)
		client.OnCmd(nil)
		//
		fmt.Println("test error done...")
	}
	res, err := client.List()
	if err != nil {
		t.Error(err)
		return
	}
	if !strings.HasPrefix(res.StrValP("/client/xxxx"), "ok->") {
		t.Error(res)
		return
	}
	if !strings.HasPrefix(res.StrValP("/slaver/master"), "ok->") {
		t.Error(res)
		return
	}
	{
		//test session is empty
		for _, c := range server.Master.L.CmdCs() {
			c.Kvs().SetVal("session", "")
		}
		_, err = client.DialSession("master", "localhost:10")
		if err == nil {
			t.Error("not dail error")
			return
		}
		_, err = client.R.Exec_m("/usr/close", util.Map{
			"sid": 100,
		})
		if err == nil {
			t.Error("not close error")
			return
		}
		// test not login
		for _, c := range server.Master.L.CmdCs() {
			c.Kvs().SetVal("ctype", "")
		}
		_, err = client.R.Exec_m("/usr/close", util.Map{
			"sid": 100,
		})
		if err == nil {
			t.Error("not close error")
			return
		}
	}
	{ //test client handler error
		runner := rc.NewRC_Runner_m_j(pool.BP, "localhost:9372", netw.NewDoNotH())
		runner.Start()
		err = runner.LoginV("abc", util.Map{
			"alias":   "xxm",
			"ctype":   TypeClient,
			"session": "xxm",
		})
		if err != nil {
			t.Error(err)
			return
		}
		runner.Stop()
		cid := server.clients["xxm"]
		// fmt.Println("---->")
		server.Send(110, TypeSlaver, cid, &mockcmd{}, []byte("abc"))
		//
	}
	{ //test slaver handler error
		runner := rc.NewRC_Runner_m_j(pool.BP, "localhost:9372", netw.NewDoNotH())
		runner.Start()
		err = runner.LoginV("abc", util.Map{
			"alias": "xxm",
			"ctype": TypeSlaver,
			"name":  "xxm",
		})
		if err != nil {
			t.Error(err)
			return
		}
		_, _, err = client.PingExec("xxm", "xxx")
		if err == nil {
			t.Error(err)
			return
		}
		runner.Stop()

	}
	time.Sleep(1 * time.Second)
	client.Close()
	server.Close()
	time.Sleep(2 * time.Second)
}

type mockwriter struct {
	runc int
}

func (m *mockwriter) Write(p []byte) (n int, err error) {
	m.runc++
	switch m.runc {
	case 1:
		err = ErrSessionClosed
	case 2:
		err = ErrSessionNotFound
	default:
		time.Sleep(2 * time.Second)
		err = fmt.Errorf("time out")
	}
	return
}

type mockcmd struct {
	netw.Cmd_
}

func (m *mockcmd) Writeb(bys ...[]byte) (n int, err error) {
	for _, b := range bys {
		n += len(b)
		os.Stdout.Write(b)
	}
	return
}
