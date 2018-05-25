package fsck

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Centny/gwf/routing"

	"github.com/Centny/gwf/routing/httptest"
)

func TestWebui(t *testing.T) {
	os.RemoveAll("/tmp/recent.json")
	master := NewMaster()
	go master.Run(":23221", map[string]int{"abc": 1})
	//
	slaver := NewSlaver("abc1")
	slaver.SP.RegisterDefaulDialer()
	err := slaver.StartSlaver("localhost:23221", "master", "abc")
	if err != nil {
		t.Error("error")
		return
	}
	time.Sleep(time.Second)
	//
	//
	master.Forward.WebSuffix = ".loc"
	webui := NewWebUI(master)
	webui.WS = "/tmp/"
	ts := httptest.NewMuxServer()
	ts.Mux.HFunc("^.*$", func(hs *routing.HTTPSession) routing.HResult {
		if hs.R.URL.Path == "/index.html" {
			hs.R.Host = "test.loc"
		}
		return routing.HRES_CONTINUE
	})
	webui.Hand(ts.Mux, "")
	//
	{ //test add/remove
		ts.G("/addForward?forwards=%v", url.QueryEscape("tcp://:1223<test>tcp://localhost:81\n"))
		ts.G("/addForward?forwards=%v", url.QueryEscape("tcp://:1224<test>tcp://localhost:81\n"))
		ts.G("/addForward?forwards=%v", url.QueryEscape("tcp://:1225<test>tcp://localhost:81\n"))
		data, _ := ts.G("/")
		if !strings.Contains(data, "tcp://:1223") {
			t.Error("error")
			return
		}
		ts.G("/removeForward?local=%v", url.QueryEscape("tcp://:1223"))
		ts.G("/addForward?forwards=%v", url.QueryEscape("tcp://:1223<test>tcp://localhost:81\n"))
		data, _ = ts.G("/")
		if !strings.Contains(data, "tcp://:1223") {
			t.Error("error")
			return
		}
		ts.G("/removeForward?local=%v", url.QueryEscape("tcp://:1223"))
		ts.G("/removeForward?local=%v", url.QueryEscape("tcp://:1224"))
		ts.G("/removeForward?local=%v", url.QueryEscape("tcp://:1225"))
		data, _ = ts.G("/")
		if !strings.Contains(data, "tcp://:1223") {
			t.Error("error")
			return
		}
		ts.G("/removeRecent?forwards=%v", url.QueryEscape("tcp://:1223<test>tcp://localhost:81"))
		data, _ = ts.G("/")
		if strings.Contains(data, "tcp://:1223") {
			t.Error("error")
			fmt.Println(data)
			return
		}
	}
	//
	{ //test auth
		master.Forward.WebAuth = "cny:sco"
		_, err := ts.G("/removeRecent")
		if err == nil {
			t.Error("error")
		}
		master.Forward.WebAuth = ""
	}
	//
	{ //test proc web
		fmt.Println(master.AllForwards())
		ts.G("/addForward?forwards=%v", url.QueryEscape("web://test<master>http://loc.m:80"))
		data, _ := ts.G("/index.html")
		if !strings.Contains(data, "nginx") {
			t.Error("error")
			fmt.Println(data)
			return
		}
		fmt.Println(data)
		fmt.Printf("-------->\n")
	}

	//
	//test cover
	ts.G("/addForward")
	ts.G("/removeForward")
	ts.G("/removeRecent")
}
