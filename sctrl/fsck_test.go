package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/sutils/fsck"
)

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

func init() {
	go runEchoServer()
}

var notZeroExit = func(code int) {
	if code == 0 {
		return
	}
	os.Exit(code)
}

var notexit = func(code int) {
}

func TestMain(t *testing.T) {
	buffered = 1024
	exitf = notZeroExit
	fsck.ShowLog = 3
	os.Remove("/tmp/.sctrl_instance.json")
	_, err := exec.Command("go", "build", ".").Output()
	if err != nil {
		t.Error(err)
		return
	}
	// os.Setenv("SCTRL_INSTANCE", "value string")
	go func() {
		tokenList.Set("abc")
		listen = ":9234"
		sctrlServer()
	}()
	time.Sleep(100 * time.Millisecond)
	go func() {
		masterAddr = "localhost:9234"
		slaverToken = "abc"
		slaverName = "test"
		webdavPath = "test"
		webdavAddr = ":9235"
		webdavUser = "test:1234"
		go sctrlWebdav()
		sctrlSlaver()
	}()
	time.Sleep(100 * time.Millisecond)
	input = make(chan []byte)
	go func() {
		webcmd, _ = os.Getwd()
		loginToken = "abc"
		wsconf = "test/.sctrl.json"
		sctrlClient()
	}()
	time.Sleep(500 * time.Millisecond)
	go func() {
		sctrlRawLogCli("debug")
	}()
	go func() {
		sctrlRawLogCli("all")
	}()
	time.Sleep(100 * time.Millisecond)
	//
	back := make(chan string)
	terminal.NotTaskCallback = back
	var writekey = func(format string, args ...interface{}) {
		input <- []byte(fmt.Sprintf("%v\necho %v$?\n", fmt.Sprintf(format, args...), terminal.CmdPrefix))
	}
	//
	writekey("shelp")
	if m := <-back; m != "0" {
		t.Error(m)
		return
	}
	{
		fmt.Println("testing add and show--->")
		//
		writekey("sadd loc2 test://root:sco@loc.m")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		writekey("sall")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		writekey("sexec echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing spick one--->")
		//
		writekey("spick loc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		writekey("sexec echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		writekey("seval test/echo.sh abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing srm--->")
		//
		writekey("srm loc2")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("sall")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("seval test/echo.sh")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing not removed---->")
		writekey("spick all")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("seval test/echo.sh")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing saddmap---->")
		writekey("saddmap echo :8392 test://localhost:9392")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("saddmap echo2 test://localhost:9392")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("slsmap")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		conn, err := net.Dial("tcp", "localhost:8392")
		if err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(conn, "test1")
		buf := make([]byte, 1024)
		_, err = conn.Read(buf)
		if string(buf[:5]) != "test1" {
			t.Error("error")
			return
		}
		conn.Close()
		writekey("srmmap echo")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		_, err = net.Dial("tcp", "localhost:8392")
		if err == nil {
			t.Error(err)
			return
		}
	}
	{
		fmt.Println("testing status---->")
		writekey("smaster")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("sprofile")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("shelp")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing eval---->")
		err = ioutil.WriteFile("/tmp/axx b.sh", []byte(`
			echo abc
			`), 0777)
		if err != nil {
			t.Error(err)
			return
		}
		writekey("seval \"/tmp/axx b.sh\"")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing scp---->")
		writekey("sscp fsck.go loc:/tmp/")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("sscp loc:/tmp/fsck.go /tmp/")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing ssh---->")
		writekey("srun wssh loc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		// time.Sleep(30 * time.Second)
		fmt.Println("testing webdav---->")
		os.Remove("/tmp/test.zip")
		os.Remove("/tmp/testa.zip")
		os.RemoveAll("/tmp/testa/")
		os.MkdirAll("/tmp/testa/", os.ModePerm)
		bys, err := exec.Command("zip", "-r", "/tmp/test.zip", ".").Output()
		if err != nil {
			fmt.Println(string(bys))
			t.Error(err)
			return
		}
		bys, err = exec.Command("curl", "-T", "/tmp/test.zip", "http://test:1234@localhost:9235/dav/test.zip").Output()
		if err != nil {
			fmt.Println(string(bys))
			t.Error(err)
			return
		}
		bys, err = exec.Command("curl", "-o", "/tmp/testa.zip", "http://test:1234@localhost:9235/dav/test.zip").Output()
		if err != nil {
			fmt.Println(string(bys))
			t.Error(err)
			return
		}
		bys, err = exec.Command("unzip", "/tmp/testa.zip", "-d", "/tmp/testa/").Output()
		if err != nil {
			fmt.Println(string(bys))
			t.Error(err)
			return
		}
		bys, err = exec.Command("curl", "-o", "/tmp/testa.zip", "http://test@localhost:9235/dav/test.zip").Output()
		if err != nil {
			fmt.Println(string(bys))
			t.Error(err)
			return
		}
		bys, err = exec.Command("curl", "-o", "/tmp/testa.zip", "http://localhost:9235/dav/test.zip").Output()
		if err != nil {
			fmt.Println(string(bys))
			t.Error(err)
			return
		}
	}
	{
		fmt.Println("testing list log---->")
		resp, err := http.Get(terminal.WebSrv.URL + "/lslog")
		if err == nil {
			_, err = ioutil.ReadAll(resp.Body)
		}
		if err != nil {
			t.Error(err)
			return
		}
	}
	{
		fmt.Println("testing switch---->")
		input <- KeyF1
		writekey("sadd loc2 root:sco@loc.m")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("sadd loc3 root:sco@loc.m connect")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("sadd loc4 root:sco@loc.m:7722")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		input <- KeyF2
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		input <- KeyF3
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		input <- KeyF4
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		input <- KeyF5
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		//
		input <- KeyF1
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		input <- KeyF6
		input <- KeyF7
		input <- KeyF8
		input <- KeyF9
		input <- KeyF10

		//
		//
		input <- KeyF1
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{ //test command usage
		input <- KeyF1
		writekey("sadd")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("sadd xxx")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("sadd xxx root:sco@loc.m:233 connect")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("srm")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("srm xxx")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		writekey("spick")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("spick xxxx")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		writekey("sexec")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("seval")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("seval /tmp/xsk.sh")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("saddmap")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("saddmap xxx")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("srmmap")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("srun wscp")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("srun wscp xx")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("sscp host1: /tmp/xx")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("sscp /tmp/xx host1:")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("sscp /tmp/xx hostxxx:/tmp")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		writekey("sscp /tmp/xx /xx/")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("srun wssh")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("srun xxxx")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("sadd loc root:sco@loc.m")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("sadd lxxxoc 'xx://%2Xroot:sc%XA%Xo@loc.m'")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
	}
	{ // not host
		writekey("spick loc2 loc3")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("srm loc2 loc3")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("seval test/echo.sh")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("srm loc loc1 loc2 loc3 loc4")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("sall")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("seval test/echo.sh")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		// return
	}
	{ //test by web command
		sctrlExec("shelp")
	}
	{ //test error
		exitf = notexit
		sctrlExec("sadd")
	}
	//
	// fmt.Fprintf(terminal, "shelp\necho %v$?\n", terminal.CmdPrefix)
	// time.Sleep(100 * time.Millisecond)
	// fmt.Fprintf(terminal, "echo %v$?\n", terminal.CmdPrefix)
	fmt.Println()
	time.Sleep(1 * time.Second)
	input <- CharESC
	input <- CharESC
	input <- CharESC
	// input <- CharTerm
	// input <- CharTerm
	// input <- CharTerm
	// input <- CharTerm
	shelpUsage.String()
	time.Sleep(4 * time.Second)
	server.Close()
	input <- nil
	tokenList.Set("abc")
	tokenList.Set("abc")
	time.Sleep(1 * time.Second)
}

func TestUsage(t *testing.T) {
	os.Args = []string{os.Args[0], "-h"}
	main()
	printClientUsage(0, true)
	printClientUsage(0, false)
	printExecUsage(0, true)
	printExecUsage(0, false)
	printLogCliUsage(0, true)
	printLogCliUsage(0, false)
	printProfileUsage(0, true)
	printProfileUsage(0, false)
	printScpUsage(0, true)
	printScpUsage(0, false)
	printServerUsage(0, true)
	printServerUsage(0, false)
	printSlaverUsage(0, true)
	printSlaverUsage(0, false)
	printSshUsage(0, true)
	printSshUsage(0, false)
}
