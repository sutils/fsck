package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Centny/gwf/netw/impl"

	"github.com/Centny/gwf/netw"

	"golang.org/x/net/webdav"

	gwflog "github.com/Centny/gwf/log"
	"github.com/Centny/gwf/routing"

	"github.com/Centny/gwf/netw/rc"

	"github.com/Centny/gwf/util"
	"github.com/sutils/fsck"
	"github.com/sutils/readkey"
)

const Version = "1.0.0"

var exitf = func(code int) {
	readkey.Close()
	os.Exit(code)
}

type ArrayFlags []string

func (a *ArrayFlags) String() string {
	return strings.Join(*a, ",")
}

func (a *ArrayFlags) Set(value string) error {
	for _, val := range *a {
		if val == value {
			return nil
		}
	}
	*a = append(*a, value)
	return nil
}

//common argument falgs
var loglevel int
var help bool
var alias bool
var webdavAddr string
var webdavPath string
var webdavUser string
var hbdelay int

//not alias argument
var runClient bool
var runServer bool
var runLogCli bool
var runExec bool
var runSsh bool
var runScp bool
var runProfile bool

func regCommonFlags() {
	flag.BoolVar(&help, "h", false, "show help")
	flag.BoolVar(&alias, "alias", false, "alias command")
	flag.IntVar(&loglevel, "loglevel", 0, "the log level")
	flag.IntVar(&hbdelay, "hbdelay", 3000, "the heartbeat delay")
	flag.StringVar(&webdavAddr, "davaddr", ":9235", "the webdav server listen address")
	flag.StringVar(&webdavPath, "davpath", "", "the webdav root path")
	flag.StringVar(&webdavUser, "davauth", "", "the webdav auth")
}

//sctrl-server argument flags
var listen string
var tokenList ArrayFlags

func regServerFlags(alias bool) {
	flag.StringVar(&listen, "listen", ":9234", "the sctrl server listen address")
	flag.Var(&tokenList, "token", "the auth token")
	if !alias {
		flag.BoolVar(&runServer, "s", false, "run as server")
	}
}

//
//sctrl-client argument
var serverAddr string
var loginToken string
var wsconf string
var bash string
var ps1 string
var instancePath string
var input chan []byte
var webcmd string
var buffered int = 1024 * 1024

func regClientFlags(alias bool) {
	flag.StringVar(&serverAddr, "server", "", "the sctrl server address")
	flag.StringVar(&loginToken, "login", "", "the token for login to server")
	flag.StringVar(&bash, "bash", "bash", "the control bash path")
	flag.StringVar(&ps1, "ps1", "Sctrl \\W>", "the bash ps1")
	flag.StringVar(&wsconf, "conf", ".sctrl.json", "the workspace configure file")
	flag.StringVar(&instancePath, "instance", "/tmp/.sctrl_instance.json", "the path to save the sctrl instance configure info")
	if !alias {
		flag.BoolVar(&runClient, "c", false, "run as client")
	}
}

//
//sctrl-slaver argument
var masterAddr string
var slaverToken string
var slaverName string

func regSlaverFlags(alias bool) {
	flag.StringVar(&masterAddr, "master", "sctrl.srv:9234", "the sctrl master server address")
	flag.StringVar(&slaverToken, "auth", "", "the token for login to server")
	flag.StringVar(&slaverName, "name", "", "the slaver name")
	if !alias {
		flag.BoolVar(&runClient, "sc", false, "run as slaver client")
	}
}

//
//sctrl-exec argument
func regExecFlags(alias bool) {
	if !alias {
		flag.BoolVar(&runExec, "run", false, "the command to execute")
	}
}

//
//sctrl-ssh argument
func regSshFlags(alias bool) {
	if !alias {
		flag.BoolVar(&runSsh, "ssh", false, "get ssh script to host")
	}
}

//
//sctrl-ssh argument
func regScpFlags(alias bool) {
	if !alias {
		flag.BoolVar(&runScp, "scp", false, "get scp script to host")
	}
}

//
//sctrl-profile argument
func regProfileFlags(alias bool) {
	if !alias {
		flag.BoolVar(&runProfile, "profile", false, "show profile")
	}
}

func printAllUsage(code int) {
	regClientFlags(false)
	regCommonFlags()
	regServerFlags(false)
	regSlaverFlags(false)
	regSshFlags(false)
	regScpFlags(false)
	regProfileFlags(false)
	regExecFlags(false)
	_, name := filepath.Split(os.Args[0])
	fmt.Fprintf(os.Stderr, "Sctrl version %v\n", Version)
	fmt.Fprintf(os.Stderr, "Usage:  %v <-s|-c|-sc|-lc|-run] [option]\n", name)
	fmt.Fprintf(os.Stderr, "        %v -s -listen :9423 -token abc\n", name)
	fmt.Fprintf(os.Stderr, "        %v -c -server sctrl.srv:9423 -login abc\n", name)
	fmt.Fprintf(os.Stderr, "        %v -sc -master sctrl.srv:9423 -auth abc -name x1\n", name)
	fmt.Fprintf(os.Stderr, "        %v -lc debug\n", name)
	fmt.Fprintf(os.Stderr, "        %v -lc host1\n", name)
	fmt.Fprintf(os.Stderr, "        %v -run sadd host root:xxx@host.local\n", name)
	fmt.Fprintf(os.Stderr, "        %v -run spick host host1\n", name)
	fmt.Fprintf(os.Stderr, "        %v -ssh host1 | bash\n", name)
	fmt.Fprintf(os.Stderr, "All options:\n")
	flag.PrintDefaults()
	exitf(code)
}

func printServerUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-server"
	}
	fmt.Fprintf(os.Stderr, "Sctrl server version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v [option] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v -listen :9423 -token abc\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -s [option]\n", name)
		fmt.Fprintf(os.Stderr, "        %v -s -listen :9423 -token abc\n", name)
	}
	fmt.Fprintf(os.Stderr, "Server options:\n")
	flag.PrintDefaults()
	exitf(code)
}

func printClientUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-client"
	}
	fmt.Fprintf(os.Stderr, "Sctrl client version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v [option] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v -server sctrl.srv:9423 -login abc\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -c [option]\n", name)
		fmt.Fprintf(os.Stderr, "        %v -c -server sctrl.srv:9423 -login abc\n", name)
	}
	fmt.Fprintf(os.Stderr, "Client options:\n")
	flag.PrintDefaults()
	exitf(code)
}

func printSlaverUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-slaver"
	}
	fmt.Fprintf(os.Stderr, "Sctrl slaver version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v [option] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v -master sctrl.srv:9423 -auth abc -name x1\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -sc [option]\n", name)
		fmt.Fprintf(os.Stderr, "        %v -sc -master sctrl.srv:9423 -auth abc -name x1\n", name)
	}
	fmt.Fprintf(os.Stderr, "Slaver options:\n")
	flag.PrintDefaults()
	exitf(code)
}

func printLogCliUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-log"
	}
	fmt.Fprintf(os.Stderr, "Sctrl log version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v [log name] [log name2] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v debug\n", name)
		fmt.Fprintf(os.Stderr, "        %v host1\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -lc [log name] [log name2] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v -lc debug\n", name)
		fmt.Fprintf(os.Stderr, "        %v -lc host1\n", name)
	}
	exitf(code)
}

func printExecUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-exec"
	}
	fmt.Fprintf(os.Stderr, "Sctrl exec version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v [command] [arg1] [arg2] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v sadd host root:xxx@host.local\n", name)
		fmt.Fprintf(os.Stderr, "        %v spick host host1\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -run [command] [arg1] [arg2] ...\n", name)
		fmt.Fprintf(os.Stderr, "        %v -run sadd host root:xxx@host.local\n", name)
		fmt.Fprintf(os.Stderr, "        %v -run spick host host1\n", name)
	}
	exitf(code)
}

func printSshUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-wssh"
	}
	fmt.Fprintf(os.Stderr, "Sctrl wssh version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v <name>\n", name)
		fmt.Fprintf(os.Stderr, "        %v host1\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -ssh <name>\n", name)
		fmt.Fprintf(os.Stderr, "        %v -ssh host1\n", name)
	}
	exitf(code)
}

func printScpUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-wscp"
	}
	fmt.Fprintf(os.Stderr, "Sctrl wscp version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v <source> <destination>\n", name)
		fmt.Fprintf(os.Stderr, "        %v ./file1 host1:/home/\n", name)
		fmt.Fprintf(os.Stderr, "        %v ./dir1 host1:/home/\n", name)
		fmt.Fprintf(os.Stderr, "        %v host1:/home/file1 /tmp/\n", name)
		fmt.Fprintf(os.Stderr, "        %v host1:/home/dir1 /tmp/\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -scp <source> <destination>\n", name)
		fmt.Fprintf(os.Stderr, "        %v -scp ./file1 host1:/home/\n", name)
		fmt.Fprintf(os.Stderr, "        %v -scp ./dir1 host1:/home/\n", name)
		fmt.Fprintf(os.Stderr, "        %v -scp host1:/home/file1 /tmp/\n", name)
		fmt.Fprintf(os.Stderr, "        %v -scp host1:/home/dir1 /tmp/\n", name)
	}
	exitf(code)
}

func printProfileUsage(code int, alias bool) {
	_, name := filepath.Split(os.Args[0])
	if alias {
		name = "sctrl-profile"
	}
	fmt.Fprintf(os.Stderr, "Sctrl profile version %v\n", Version)
	if alias {
		fmt.Fprintf(os.Stderr, "Usage:  %v\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "Usage:  %v -profile`\n", name)
	}
	exitf(code)
}

func main() {
	_, name := filepath.Split(os.Args[0])
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch {
	case name == "sctrl-srv" || name == "sctrl-server" || mode == "-s":
		regCommonFlags()
		regServerFlags(name == "sctrl-server")
		flag.Parse()
		if help {
			printServerUsage(0, alias || name == "sctrl-server")
		}
		if len(listen) < 1 || len(tokenList) < 1 {
			printServerUsage(1, alias || name == "sctrl-server")
		}
		go sctrlWebdav()
		sctrlServer()
	case name == "sctrl-cli" || name == "sctrl-client" || mode == "-c":
		regCommonFlags()
		regClientFlags(name == "sctrl-client")
		flag.Parse()
		if help {
			printClientUsage(0, alias || name == "sctrl-client")
		}
		go sctrlWebdav()
		sctrlClient()
	case name == "sctrl-sc" || name == "sctrl-slaver" || mode == "-sc":
		regCommonFlags()
		regSlaverFlags(name == "sctrl-slaver")
		flag.Parse()
		if help {
			printSlaverUsage(0, alias || name == "sctrl-slaver")
		}
		if len(masterAddr) < 1 || len(slaverToken) < 1 || len(slaverName) < 1 {
			flag.Usage()
			exitf(1)
		}
		go sctrlWebdav()
		sctrlSlaver()
	case name == "sctrl-log" || mode == "-lc":
		for _, arg := range os.Args {
			if arg == "-h" {
				printLogCliUsage(0, alias || name == "sctrl-log")
			} else if arg == "-alias" {
				alias = true
			}
		}
		if mode == "-lc" {
			if len(os.Args) < 3 {
				printLogCliUsage(1, alias || name == "sctrl-log")
			}
			sctrlLogCli(os.Args[2:]...)
		} else {
			if len(os.Args) < 2 {
				printLogCliUsage(1, alias || name == "sctrl-log")
			}
			sctrlLogCli(os.Args[1:]...)
		}
	case name == "sctrl-exec" || mode == "-run":
		for _, arg := range os.Args {
			if arg == "-h" {
				printExecUsage(0, alias || name == "sctrl-exec")
			} else if arg == "-alias" {
				alias = true
			}
		}
		if mode == "-run" {
			if len(os.Args) < 3 {
				printExecUsage(1, alias || name == "sctrl-exec")
			}
			sctrlExec(JoinArgs("", os.Args[2:]...))
		} else {
			if len(os.Args) < 2 {
				printExecUsage(1, alias || name == "sctrl-exec")
			}
			sctrlExec(JoinArgs("", os.Args[1:]...))
		}
	case name == "sctrl-wssh" || mode == "-ssh":
		for _, arg := range os.Args {
			if arg == "-h" {
				printSshUsage(0, alias || name == "sctrl-wssh")
			}
		}
		var cmds string
		if mode == "-ssh" {
			if len(os.Args) < 3 {
				printSshUsage(0, name == "sctrl-wssh")
			}
			cmds = JoinArgs("wssh", os.Args[2])
		} else {
			if len(os.Args) < 2 {
				printSshUsage(0, name == "sctrl-wssh")
			}
			cmds = JoinArgs("wssh", os.Args[1])
		}
		code, err := execCmds(cmds, false, true, true)
		if err != nil {
			code = -1
			fmt.Fprintf(os.Stdout, "echo %v && exit %v\n", err, code)
		}
		exitf(code)
	case name == "sctrl-wscp" || mode == "-scp":
		for _, arg := range os.Args {
			if arg == "-h" {
				printScpUsage(0, alias || name == "sctrl-wscp")
			}
		}
		var cmds string
		if mode == "-scp" {
			if len(os.Args) < 4 {
				printScpUsage(0, alias || name == "sctrl-wscp")
			}
			cmds = JoinArgs("wscp", os.Args[2:]...)
		} else {
			if len(os.Args) < 3 {
				printScpUsage(0, alias || name == "sctrl-wscp")
			}
			fmt.Println(os.Args)
			cmds = JoinArgs("wscp", os.Args[1:]...)
		}
		code, err := execCmds(cmds, false, false, true)
		if err != nil {
			code = -1
			fmt.Fprintf(os.Stdout, "echo %v && exit %v\n", err, code)
		}
		exitf(code)
	case name == "sctrl-profile" || mode == "-profile":
		for _, arg := range os.Args {
			if arg == "-h" {
				printProfileUsage(0, alias || name == "sctrl-profile")
			} else if arg == "-alias" {
				alias = true
			}
		}
		sctrlExec("profile")
	case mode == "-h":
		printAllUsage(0)
	default:
		printAllUsage(1)
	}
}

var server *fsck.Server

func sctrlServer() {
	log.Printf("start sctrl server by listen(%v),loglevel(%v),token(%v)", listen, loglevel, tokenList)
	fsck.ShowLog = loglevel
	netw.ShowLog = loglevel > 2
	impl.ShowLog = loglevel > 3
	//
	//
	tokens := map[string]int{}
	for _, token := range tokenList {
		tokens[token] = 1
	}
	server = fsck.NewServer()
	server.HbDelay = int64(hbdelay)
	err := server.Run(listen, tokens)
	if err != nil {
		fmt.Println(err)
		exitf(1)
	} else {
		exitf(0)
	}
}

func sctrlSlaver() {
	fsck.ShowLog = loglevel
	netw.ShowLog = loglevel > 2
	impl.ShowLog = loglevel > 3
	slaver := fsck.NewSlaver("slaver")
	slaver.HbDelay = int64(hbdelay)
	slaver.StartSlaver(masterAddr, slaverName, slaverToken)
	wait := make(chan int)
	<-wait
	exitf(0)
}

var terminal *Terminal

func sctrlClient() {
	fsck.ShowLog = loglevel
	netw.ShowLog = loglevel > 2
	impl.ShowLog = loglevel > 3
	var err error
	var conf = &WorkConf{}
	var client *fsck.Slaver
	var name = "Sctrl"
	if len(serverAddr) < 1 {
		conf, err = ReadWorkConf(wsconf)
		if err != nil {
			fmt.Printf("read %v fail, %v", wsconf, err)
			exitf(1)
		}
		serverAddr = conf.SrvAddr
		if len(serverAddr) < 1 {
			fmt.Println("server config is empty")
			flag.Usage()
			exitf(1)
		}
		if len(conf.Login) > 0 {
			loginToken = conf.Login
		}
		if len(conf.PS1) > 0 {
			ps1 = conf.PS1
		}
		if len(conf.Bash) > 0 {
			bash = conf.Bash
		}
		if len(conf.Instance) > 0 {
			instancePath = conf.Instance
		}
		if len(conf.Name) > 0 {
			name = conf.Name
		}
	}
	if len(loginToken) < 1 {
		for {
			fmt.Printf("Login to %v: ", serverAddr)
			time.Sleep(100 * time.Millisecond)
			readkey.Open()
			buf := []byte{}
			for {
				key, err := readkey.Read()
				if err != nil || bytes.Equal(key, CharTerm) {
					readkey.Close()
					exitf(1)
				}
				if key[0] == '\r' {
					fmt.Println()
					break
				} else if key[0] == 127 && len(buf) > 0 { //delete
					buf = buf[0 : len(buf)-1]
					continue
				}
				buf = append(buf, key...)
			}
			loginToken = strings.TrimSpace(string(buf))
			if len(loginToken) > 0 {
				break
			}
		}
	}
	fmt.Printf("start %v by:\n  server:%v\n  bash:%v\n  ps1:%v\n  instance:%v\n",
		name, serverAddr, bash, ps1, instancePath)
	//
	login := make(chan int)
	client = fsck.NewSlaver("client")
	client.HbDelay = int64(hbdelay)
	client.OnLogin = func(a *rc.AutoLoginH, err error) {
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("\nlogin to %v fail with %v\n", serverAddr, err)
			exitf(1)
		}
		login <- 1
	}
	if len(webcmd) < 1 {
		exepath, _ := os.Executable()
		exepath, _ = filepath.Abs(exepath)
		webcmd, _ = filepath.Split(exepath)
	}
	terminal = NewTerminal(client, name, ps1, bash, webcmd, input == nil, buffered)
	terminal.InstancePath = instancePath
	for key, val := range conf.Env {
		terminal.Env = append(terminal.Env, fmt.Sprintf("%v=%v", key, val))
	}

	logout := NewNamedWriter("debug", terminal.Log)
	log.SetOutput(logout)
	gwflog.SetWriter(logout)
	//
	fmt.Printf("start connect to %v\n", serverAddr)
	err = client.StartClient(serverAddr, util.UUID(), loginToken)
	if err != nil {
		fmt.Println(err)
		exitf(1)
	}
	fmt.Printf("%v is connected\n", serverAddr)
	<-login
	terminal.Start(conf)
	if input == nil {
		terminal.ProcReadkey()
		exitf(0)
		return
	}
	for key := range input {
		if key == nil {
			break
		}
		terminal.Write(key)
	}
	exitf(0)
}

func sctrlExec(cmds string) {
	code, err := execCmds(cmds, true, false, false)
	if err != nil {
		fmt.Printf("-error: %v\n", err)
		code = -1
	}
	if code == 200 {
		code = 0
	}
	exitf(code)
}

func execCmds(cmds string, log, wait, single bool) (code int, err error) {
	url, _, err := findWebURL("", log, wait, single, 5*time.Second)
	if err == nil {
		code, err = ExecWebCmd(url+"/exec", cmds, os.Stdout)
	}
	return
}

func sctrlRawLogCli(name ...string) {
	var url, last string
	var err error
	delay := 5 * time.Second
	ns := strings.Join(name, ",")
	logw := NewWebLogPrinter(os.Stdout)
	for {
		url, last, err = findWebURL(last, true, true, false, delay)
		if err != nil { //having error.
			fmt.Println(err)
			exitf(1)
		}
		_, err := ExecWebLog(url+"/log", ns, "1", logw)
		if err != nil && reflect.TypeOf(err).String() == "*url.Error" && len(last) > 0 {
			fmt.Printf("->last instance(%v) is offline, will try after %v\n", last, delay)
		} else {
			fmt.Printf("->reply error(%v):%v\n", reflect.TypeOf(err), err)
		}
		time.Sleep(delay)
	}
}

func sctrlLogCli(name ...string) {
	var url, last string
	var err error
	delay := 5 * time.Second
	ns := strings.Join(name, ",")
	logw := NewWebLogPrinter(os.Stdout)
	url, last, _ = findWebURL(last, true, true, false, delay)
	//
	pre := "1"
	done := make(chan int)
	switching := false
	var runlog = func() {
		fmt.Printf("\n\n------------------ %v ------------------\n", ns)
		for !switching {
			url, last, err = findWebURL(last, true, true, false, delay)
			if err != nil { //having error.
				fmt.Println(err)
				exitf(1)
			}
			_, err := ExecWebLog(url+"/log", ns, pre, logw)
			if switching {
				continue
			}
			if err != nil && reflect.TypeOf(err).String() == "*url.Error" && len(last) > 0 {
				fmt.Printf("->last instance(%v) is offline, will try after %v\n", last, delay)
			} else {
				fmt.Printf("->reply error(%v):%v\n", reflect.TypeOf(err), err)
			}
			time.Sleep(delay)
		}
		pre = "0"
		done <- 1
	}
	go runlog()
	var buf []byte
	var idxSwitch = func(idx int) {
		fmt.Printf("\nWaiting log %v stop...\n", ns)
		switching = true
		old := logw
		logw = NewWebLogPrinter(os.Stdout)
		old.Close()
		var bys []byte
		resp, err := http.Get(url + "/lslog")
		if err == nil {
			bys, err = ioutil.ReadAll(resp.Body)
		}
		<-done
		if err != nil {
			fmt.Printf("list log name fail with %v\n", err)
			return
		}
		fmt.Printf("\nSupported:\n")
		os.Stdout.Write(bys)
		rawns := SpaceRegex.Split(string(bys), -1)
		allns := []string{}
		for _, n := range rawns {
			if len(n) < 1 {
				continue
			}
			allns = append(allns, n)
		}
		if idx >= len(allns) {
			fmt.Printf("\n->switch fail with index out of bound by %v, rollback to %v\n", idx, ns)
			time.Sleep(2 * time.Second)
		} else {
			ns = allns[idx]
		}
		switching = false
		go runlog()
	}
	readkey.Open()
	for {
		key, err := readkey.Read()
		if err != nil || bytes.Equal(key, CharTerm) {
			fmt.Println()
			break
		}
		if key[0] == 127 { //delete
			if len(buf) > 0 {
				buf = buf[0 : len(buf)-1]
				os.Stdout.WriteString("\b \b")
			}
			continue
		}
		switch {
		case bytes.Equal(key, KeyF1):
			idxSwitch(0)
		case bytes.Equal(key, KeyF2):
			idxSwitch(1)
		case bytes.Equal(key, KeyF3):
			idxSwitch(2)
		case bytes.Equal(key, KeyF4):
			idxSwitch(3)
		case bytes.Equal(key, KeyF5):
			idxSwitch(4)
		case bytes.Equal(key, KeyF6):
			idxSwitch(5)
		case bytes.Equal(key, KeyF7):
			idxSwitch(6)
		case bytes.Equal(key, KeyF8):
			idxSwitch(7)
		case bytes.Equal(key, KeyF9):
			idxSwitch(8)
		case bytes.Equal(key, KeyF10):
			idxSwitch(9)
		default:
			if key[0] != '\r' {
				buf = append(buf, key...)
				os.Stdout.Write(key)
				continue
			}
			if switching {
				if len(buf) < 1 {
					fmt.Printf("\nPlease entry log name:")
					continue
				}
				ns = strings.TrimSpace(string(buf))
				switching = false
				go runlog()
				buf = nil
			} else {
				fmt.Printf("\nWaiting log %v stop...\n", ns)
				switching = true
				old := logw
				logw = NewWebLogPrinter(os.Stdout)
				old.Close()
				resp, err := http.Get(url + "/lslog")
				<-done
				if err != nil {
					fmt.Printf("list log name fail with %v\n", err)
				} else {
					fmt.Printf("\nSupported:\n")
					io.Copy(os.Stdout, resp.Body)
					resp.Body.Close()
					fmt.Println()
				}
				fmt.Printf("\nPlease entry log name:")
				buf = nil
			}
		}
	}
	exitf(0)
}

func sctrlWebdav() {
	if len(webdavPath) < 1 {
		return
	}
	webdav := &webdav.Handler{
		Prefix:     "/dav",
		FileSystem: webdav.Dir(webdavPath),
		LockSystem: webdav.NewMemLS(),
		Logger: func(req *http.Request, err error) {
			if err == nil {
				gwflog.D("Dav %v to %v success", req.Method, req.URL.Path)
			} else {
				gwflog.E("Dav %v to %v error %v", req.Method, req.URL.Path, err)
			}
		},
	}
	routing.Shared.HFilterFunc("^/.*$", func(hs *routing.HTTPSession) routing.HResult {
		if len(webdavUser) > 0 {
			usr, pwd, ok := hs.R.BasicAuth()
			if !ok || fmt.Sprintf("%v:%v", usr, pwd) != webdavUser {
				hs.W.WriteHeader(403)
				hs.W.Write([]byte("not access\n"))
				return routing.HRES_RETURN
			}
		}
		return routing.HRES_CONTINUE
	})
	routing.Shared.Handler("^/dav/.*$", webdav)
	log.Printf("start webdav server by listen(%v),davpth(%v)", webdavAddr, webdavPath)
	err := routing.ListenAndServe(webdavAddr)
	fmt.Println(err)
	exitf(1)
}

func findWebURL(last string, log, wait, signle bool, delay time.Duration) (url string, pwd string, err error) {
	url = os.Getenv(KeyWebCmdURL)
	if len(url) > 0 {
		return
	}
	var data []byte
	var confPath string
	var oneconf map[string]interface{}
	allConf := []string{
		filepath.Join(os.Getenv("HOME"), ".sctrl_instance.json"),
		filepath.Join(os.Getenv("TMPDIR"), ".sctrl_instance.json"),
		filepath.Join("/tmp", ".sctrl_instance.json"),
	}
	instance := os.Getenv("SCTRL_INSTANCE")
	for {
		for _, confPath = range allConf {
			data, err = ioutil.ReadFile(confPath)
			if err == nil {
				break
			}
		}
		if data == nil {
			if wait {
				if log {
					fmt.Printf("->instance conf is not found on %v, will try after %v\n", allConf, delay)
				}
				time.Sleep(delay)
				continue
			}
			err = fmt.Errorf("find the fsck web url fail")
			return
		}
		rawConfList := []util.Map{}
		err = json.Unmarshal(data, &rawConfList)
		if err != nil {
			err = fmt.Errorf("read fsck config file(%v) fail with %v", confPath, err)
			return
		}
		confList := []util.Map{}
		now := util.Now()
		maxname := 0
		for _, conf := range rawConfList {
			last := conf.IntValV("last", 0)
			if last < 1 || now-last > 5000 {
				continue
			}
			name := conf.StrVal("name")
			if len(instance) > 0 && name != instance {
				continue
			}
			nlen := len(name)
			if nlen > maxname {
				maxname = nlen
			}
			confList = append(confList, conf)
		}
		if len(confList) < 1 {
			//configure not found
			if wait {
				if log {
					fmt.Printf("->instance is not found on %v, will try after %v\n", confPath, delay)
				}
				time.Sleep(delay)
				continue
			}
			err = fmt.Errorf("not running instance")
			return
		}
		if len(last) > 0 {
			//the pwd is specified
			for _, conf := range confList {
				pwd = conf.StrVal("pwd")
				if pwd == last {
					oneconf = conf
					break
				}
			}
			if oneconf != nil {
				break
			}
			//last not foud
			if wait {
				if log {
					fmt.Printf("->last instance(%v) is offline, will try after %v\n", last, delay)
				}
				time.Sleep(delay)
				continue
			}
			err = fmt.Errorf("not instance for pwd(%v)", last)
			return
		}
		if len(confList) == 1 {
			//only one
			oneconf = confList[0]
			break
		}
		if signle {
			err = fmt.Errorf("more than one instance found, please pick one by export SCTRL_INSTANCE")
			return
		}
		// //check whether current dir is the workspace.
		// wd, _ := os.Getwd()
		// for _, conf := range confList {
		// 	pwd = conf.StrVal("pwd")
		// 	if pwd == wd {
		// 		oneconf = conf
		// 		break
		// 	}
		// }
		// if oneconf != nil {
		// 	break
		// }
		//create instance info log.
		buf := bytes.NewBuffer(nil)
		format := fmt.Sprintf("%v%v%v", "%3d %", maxname, "s %v\n")
		for idx, conf := range confList {
			fmt.Fprintf(buf, format, idx, conf["name"], conf["pwd"])
		}
		//
		var rbuf = make([]byte, 1024)
		var key string
		for {
			buf.WriteTo(os.Stdout)
			fmt.Fprintf(os.Stdout, "Please select one(type r to reload)[0]:")
			readed, _ := os.Stdin.Read(rbuf)
			key = strings.TrimSpace(string(rbuf[:readed]))
			if key == "r" {
				//reload instance info from file.
				break
			}
			if key == "\n" {
				//reshow the read instance info
				continue
			}
			idx, perr := strconv.ParseUint(key, 10, 32)
			if perr == nil {
				//select by index.
				if len(confList) < int(idx) {
					fmt.Fprintf(os.Stdout, "-error: %v index out of bounds\n", idx)
					continue
				}
				oneconf = confList[idx]
				break
			}
			//select by name
			for _, conf := range confList {
				name := fmt.Sprintf("%v", conf["name"])
				if name == key {
					oneconf = conf
					break
				}
			}
			if oneconf == nil {
				fmt.Fprintf(os.Stdout, "-error: %v name not found\n", key)
				continue
			} else {
				break
			}
		}
		if key == "r" {
			//want to reload
			continue
		}
		if oneconf != nil {
			break
		}
	}
	var ok bool
	pwd, ok = oneconf["pwd"].(string)
	url, ok = oneconf["web_url"].(string)
	if !ok {
		err = fmt.Errorf("read fsck config file(%v),instance(%v) fail with %v", confPath, oneconf["pwd"], "web_url not configured")
	}
	return
}
