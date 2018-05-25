package fsck

import (
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/Centny/gwf/routing"
	"github.com/Centny/gwf/routing/httptest"

	"github.com/Centny/gwf/util"
)

func TestCmdDialer(t *testing.T) {
	cmd := NewCmdDialer()
	cmd.Bootstrap()
	if !cmd.Matched("tcp://cmd?exec=bash") {
		t.Error("error")
		return
	}
	raw, err := cmd.Dial(10, "tcp://cmd?exec=bash")
	if err != nil {
		t.Error(err)
		return
	}
	go io.Copy(os.Stdout, raw)
	fmt.Fprintf(raw, "ls\n")
	fmt.Fprintf(raw, "ls /tmp/\n")
	fmt.Fprintf(raw, "echo abc\n")
	time.Sleep(200 * time.Millisecond)
	raw.Write(CMD_CTRL_C)
	time.Sleep(200 * time.Millisecond)
	raw.Close()
	time.Sleep(200 * time.Millisecond)
	//for cover
	fmt.Printf("%v\n", cmd)
	//
	//test encoding
	raw, err = cmd.Dial(10, "tcp://cmd?exec=bash&LC=zh_CN.GBK")
	if err != nil {
		t.Error(err)
		return
	}
	go io.Copy(os.Stdout, raw)
	fmt.Fprintf(raw, "ls\n")
	time.Sleep(200 * time.Millisecond)
	raw.Close()
	//
	raw, err = cmd.Dial(10, "tcp://cmd?exec=bash&LC=zh_CN.GB18030")
	if err != nil {
		t.Error(err)
		return
	}
	go io.Copy(os.Stdout, raw)
	fmt.Fprintf(raw, "ls\n")
	time.Sleep(200 * time.Millisecond)
	raw.Close()
	//
	//test error
	_, err = cmd.Dial(100, "://cmd")
	if err == nil {
		t.Error("error")
		return
	}
}

func TestWebDialer(t *testing.T) {
	//test web dialer
	l, err := net.Listen("tcp", ":2422")
	if err != nil {
		t.Error(err)
		return
	}
	dialer := NewWebDialer()
	dialer.Bootstrap()
	go func() {
		var cid uint16
		for {
			con, err := l.Accept()
			if err != nil {
				break
			}
			raw, err := dialer.Dial(cid, "tcp://web?dir=/tmp")
			if err != nil {
				panic(err)
			}
			go func() {
				buf := make([]byte, 1024)
				for {
					n, err := raw.Read(buf)
					if err != nil {
						con.Close()
						break
					}
					con.Write(buf[0:n])
				}
			}()
			go io.Copy(raw, con)
		}
	}()
	fmt.Println(util.HGet("http://localhost:2422/"))
	fmt.Println(util.HPost("http://localhost:2422/", nil))
	dialer.Shutdown()
	time.Sleep(100 * time.Millisecond)
	//for cover
	fmt.Printf("%v,%v\n", dialer.Addr(), dialer.Network())
	//test web conn
	conn, _, err := PipeWebDialerConn(100, "tcp://web?dir=/tmp")
	if err != nil {
		t.Error(err)
		return
	}
	conn.SetDeadline(time.Now())
	conn.SetReadDeadline(time.Now())
	conn.SetWriteDeadline(time.Now())
	fmt.Printf("%v,%v,%v\n", conn.LocalAddr(), conn.RemoteAddr(), conn.Network())
	//test error
	_, _, err = PipeWebDialerConn(100, "://")
	if err == nil {
		t.Error(err)
		return
	}
	_, _, err = PipeWebDialerConn(100, "tcp://web")
	if err == nil {
		t.Error(err)
		return
	}
	//
	ts := httptest.NewServer(func(hs *routing.HTTPSession) routing.HResult {
		dialer.ServeHTTP(hs.W, hs.R)
		return routing.HRES_RETURN
	})
	data, err := ts.G("/")
	if err == nil {
		t.Errorf("%v-%v", data, err)
		return
	}
	//
	_, err = dialer.Dial(100, "://")
	if err == nil {
		t.Error(err)
		return
	}
}
