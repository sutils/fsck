package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Centny/gwf/util"
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

func RKClose(n string) {

}

func RKOpen(n string) {

}

var rkinputCli = make(chan []byte)
var rkinputLog = make(chan []byte)
var rkinputLogin = make(chan []byte, 10)
var rkinputDefault = make(chan []byte)

func RKRead(n string) (key []byte, err error) {
	switch n {
	case "cli":
		key = <-rkinputCli
	case "log":
		key = <-rkinputLog
	case "login":
		key = <-rkinputLogin
	default:
		key = <-rkinputDefault
	}
	if key == nil {
		err = io.EOF
	}
	return
}

func RKGetSieze() (w, h int) {
	return 80, 60
}

func RKSetSize(fd uintptr, w, h int) (err error) {
	return
}

func init() {
	go runEchoServer()
	readkeyClose = RKClose
	readkeyGetSize = RKGetSieze
	readkeyOpen = RKOpen
	readkeyRead = RKRead
	readkeySetSize = RKSetSize
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
	go func() {
		sctrlLogCli("all")
	}()
	time.Sleep(2 * time.Second)
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
		realAddr = ":9235"
		go sctrlWebdav()
		go sctrlReal()
		sctrlSlaver()
	}()
	time.Sleep(100 * time.Millisecond)
	go func() {
		webcmd, _ = os.Getwd()
		wsconf = "test/.sctrl.json"
		rkinputLogin <- []byte("abc")
		rkinputLogin <- []byte{127}
		rkinputLogin <- []byte("c")
		rkinputLogin <- []byte("\r")
		sctrlClient()
	}()
	time.Sleep(500 * time.Millisecond)
	//
	back := make(chan string)
	terminal.NotTaskCallback = back
	var writekey = func(format string, args ...interface{}) {
		rkinputCli <- []byte(fmt.Sprintf("%v\necho %v$?\n", fmt.Sprintf(format, args...), terminal.CmdPrefix))
	}
	{
		//test switch
		rkinputCli <- CharCtrlb
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- CharEnter
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- []byte("loc")
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- CharDelete
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- []byte("c")
		time.Sleep(time.Second)
		rkinputCli <- CharEnter
		time.Sleep(5 * time.Second)
		//
		rkinputCli <- CharCtrlb
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- []byte("sctrl")
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- CharEnter
		time.Sleep(time.Second)
		//
		rkinputCli <- CharCtrlb
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- []byte("xxxxxx")
		time.Sleep(500 * time.Millisecond)
		rkinputCli <- CharEnter
		time.Sleep(time.Second)
		//
		rkinputCli <- KeyF1
		time.Sleep(2 * time.Second)
	}
	{
		fmt.Println("testing sping---->")
		rkinputCli <- []byte("sping test\n")
		time.Sleep(3 * time.Second)
		rkinputCli <- CharTerm
		rkinputCli <- CharTerm
		time.Sleep(2 * time.Second)
	}
	{
		fmt.Println("testing kill---->")
		rkinputCli <- []byte("sexec sleep 10\n")
		time.Sleep(2 * time.Second)
		rkinputCli <- CharTerm
		rkinputCli <- CharTerm
		time.Sleep(2 * time.Second)
	}
	//
	{ //test log cli
		//
		fmt.Println("test log cli by name--->")
		rkinputLog <- []byte{'\r'}
		time.Sleep(time.Second)
		rkinputLog <- []byte{'\r'}
		time.Sleep(time.Second)
		rkinputLog <- []byte("all")
		time.Sleep(500 * time.Millisecond)
		rkinputLog <- []byte{127}
		time.Sleep(1000 * time.Millisecond)
		rkinputLog <- []byte{'l'}
		time.Sleep(1000 * time.Millisecond)
		rkinputLog <- []byte{'\r'}
		time.Sleep(1 * time.Second)

		//
		fmt.Println("test log cli by index--->")
		rkinputLog <- KeyF10
		fmt.Println("key10 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF9
		fmt.Println("key9 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF8
		fmt.Println("key8 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF7
		fmt.Println("key7 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF6
		fmt.Println("key6 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF5
		fmt.Println("key5 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF4
		fmt.Println("key4 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF3
		fmt.Println("key3 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF2
		fmt.Println("key2 done")
		time.Sleep(time.Second)
		rkinputLog <- KeyF1
		fmt.Println("key1 done")
		time.Sleep(2 * time.Second)
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
		writekey("saddmap echo 'tcp://:8392<test>tcp://localhost:9392'")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		writekey("saddmap echo2 'tcp://<test>tcp://localhost:9392'")
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
		fmt.Println("testing sslaver---->")
		writekey("sslaver test")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{
		fmt.Println("testing sreal---->")
		_, err = fsck.NotifyReal("http://localhost:9235/real/update", util.Map{
			"c1": util.Map{
				"abcdefg": 1,
				"y":       1,
				"z":       1,
			},
		})
		if err != nil {
			t.Error(err)
			return
		}
		rkinputCli <- []byte("sreal test abcdefg y z=avg -timeout=1 -delay=1 -host=c1\n")
		time.Sleep(3 * time.Second)
		rkinputCli <- CharTerm
		rkinputCli <- CharTerm
		time.Sleep(time.Second)
		rkinputCli <- []byte("sreal test -clear\n")
		time.Sleep(3 * time.Second)
		rkinputCli <- CharTerm
		rkinputCli <- CharTerm
		time.Sleep(time.Second)
	}
	{
		fmt.Println("testing switch---->")
		rkinputCli <- KeyF1
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
		rkinputCli <- KeyF2
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		rkinputCli <- KeyF3
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		rkinputCli <- KeyF4
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		rkinputCli <- KeyF5
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		//
		//
		rkinputCli <- KeyF1
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
		rkinputCli <- KeyF6
		rkinputCli <- KeyF7
		rkinputCli <- KeyF8
		rkinputCli <- KeyF9
		rkinputCli <- KeyF10

		//
		//
		rkinputCli <- KeyF1
		writekey("echo abc")
		if m := <-back; m != "0" {
			t.Error(m)
			return
		}
	}
	{ //test command usage
		rkinputCli <- KeyF1
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
		//
		writekey("sping")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("sexec sterm")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("sreal")
		if m := <-back; m == "0" {
			t.Error(m)
			return
		}
		//
		writekey("sslaver")
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
		//
		fmt.Println("testing not host list log---->")
		resp, err := http.Get(terminal.WebSrv.URL + "/lslog")
		var bys []byte
		if err == nil {
			bys, err = ioutil.ReadAll(resp.Body)
		}
		os.Stdout.Write(bys)
		if err != nil {
			t.Error(err)
			return
		}
		// return
	}
	{ //test by web command
		sctrlExec("shelp", nil, true)
		wait := make(chan int)
		go func() {
			sctrlExec("sexec sleep 10", nil, true)
			wait <- 0
		}()
		for ctrlcSig == nil {
			time.Sleep(time.Second)
			fmt.Println("ctrlc is nil")
		}
		time.Sleep(time.Second)
		ctrlcSig <- os.Interrupt
		<-wait
	}
	{ //test error
		exitf = notexit
		sctrlExec("sadd", nil, true)
	}
	//
	// fmt.Fprintf(terminal, "shelp\necho %v$?\n", terminal.CmdPrefix)
	// time.Sleep(100 * time.Millisecond)
	// fmt.Fprintf(terminal, "echo %v$?\n", terminal.CmdPrefix)
	fmt.Println()
	time.Sleep(1 * time.Second)
	rkinputCli <- CharESC
	rkinputCli <- CharESC
	rkinputCli <- CharESC
	// input <- CharTerm
	// input <- CharTerm
	// input <- CharTerm
	// input <- CharTerm
	fmt.Println(shelpUsage.String())
	time.Sleep(4 * time.Second)
	server.Close()
	//rkinput <- nil
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
