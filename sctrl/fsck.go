package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Centny/gwf/netw/impl"

	"github.com/Centny/gwf/netw"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/webdav"

	"github.com/sutils/readkey"

	gwflog "github.com/Centny/gwf/log"
	"github.com/Centny/gwf/routing"

	"github.com/Centny/gwf/netw/rc"

	"github.com/Centny/gwf/util"
	"github.com/sutils/fsck"
)

const Version = "1.0.0"

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

func printAllUsage(code int) {
	regClientFlags(false)
	regCommonFlags()
	regServerFlags(false)
	regSlaverFlags(false)
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
	fmt.Fprintf(os.Stderr, "All options:\n")
	flag.PrintDefaults()
	os.Exit(code)
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
	os.Exit(code)
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
	os.Exit(code)
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
	os.Exit(code)
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
	os.Exit(code)
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
	os.Exit(code)
}

func main() {
	_, name := filepath.Split(os.Args[0])
	mode := ""
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch {
	case mode == "-sim":
		host, err := ParseSshHost("xx", "test://root:Asd_321@222.16.80.120:10022?pty=xterm", nil)
		if err != nil {
			panic(err)
		}
		session := NewSshSession(nil, host)
		conn, err := net.Dial("tcp", "222.16.80.120:10022")
		if err != nil {
			panic(err)
		}
		defer readkey.Close()
		session.Add(os.Stdout)
		session.StartSession(conn)
		for {
			key, err := readkey.ReadKey()
			if err != nil || bytes.Equal(key, CharTerm) {
				break
			}
			fmt.Printf("-->%v\n", key)
			session.Write(key)
		}
	case mode == "-sim2":
		// Create client config
		config := &ssh.ClientConfig{
			User: "root",
			Auth: []ssh.AuthMethod{
				ssh.Password("Asd_321"),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		// Connect to ssh server
		conn, err := ssh.Dial("tcp", "222.16.80.120:10022", config)
		if err != nil {
			log.Fatal("unable to connect: ", err)
		}
		defer conn.Close()
		// Create a session
		session, err := conn.NewSession()
		if err != nil {
			log.Fatal("unable to create session: ", err)
		}
		session.Stdout = os.Stdout
		session.Stderr = os.Stderr
		stdin, _ := session.StdinPipe()
		defer session.Close()
		// Set up terminal modes
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,     // disable echoing
			ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
			ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
		}
		// Request pseudo terminal
		if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
			log.Fatal("request for pseudo terminal failed: ", err)
		}
		// Start remote shell
		if err := session.Shell(); err != nil {
			log.Fatal("failed to start shell: ", err)
		}
		for {
			key, err := readkey.ReadKey()
			if err != nil || bytes.Equal(key, CharTerm) {
				break
			}
			fmt.Printf("-->%v\n", key)
			stdin.Write(key)
		}
		session.Wait()
	case name == "sctrl-server" || mode == "-s":
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
	case name == "sctrl-client" || mode == "-c":
		regCommonFlags()
		regClientFlags(name == "sctrl-client")
		flag.Parse()
		if help {
			printClientUsage(0, alias || name == "sctrl-client")
		}
		go sctrlWebdav()
		sctrlClient()
	case name == "sctrl-slaver" || mode == "-sc":
		regCommonFlags()
		regSlaverFlags(name == "sctrl-slaver")
		flag.Parse()
		if help {
			printSlaverUsage(0, alias || name == "sctrl-slaver")
		}
		if len(masterAddr) < 1 || len(slaverToken) < 1 || len(slaverName) < 1 {
			flag.Usage()
			os.Exit(1)
		}
		go sctrlWebdav()
		sctrlSlaver()
	case name == "sctrl-log" || mode == "-lc":
		for _, arg := range os.Args {
			if arg == "-h" {
				printLogCliUsage(0, alias || name == "sctrl-server")
			} else if arg == "-alias" {
				alias = true
			}
		}
		if mode == "-lc" {
			if len(os.Args) < 3 {
				printLogCliUsage(1, alias || name == "sctrl-server")
			}
			sctrlLogCli(os.Args[2:]...)
		} else {
			if len(os.Args) < 2 {
				printLogCliUsage(1, alias || name == "sctrl-server")
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
	case mode == "-h":
		printAllUsage(0)
	default:
		printAllUsage(1)
	}
}

func sctrlServer() {
	log.Printf("start sctrl server by listen(%v),loglevel(%v),token(%v)", listen, loglevel, tokenList)
	fsck.ShowLog = loglevel
	netw.ShowLog = loglevel > 0
	impl.ShowLog = loglevel > 0
	//
	//
	tokens := map[string]int{}
	for _, token := range tokenList {
		tokens[token] = 1
	}
	server := fsck.NewServer()
	err := server.Run(listen, tokens)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println(err)
}

func sctrlSlaver() {
	fsck.ShowLog = loglevel
	netw.ShowLog = loglevel > 0
	impl.ShowLog = loglevel > 0
	slaver := fsck.NewSlaver("slaver")
	slaver.HbDelay = int64(hbdelay)
	slaver.StartSlaver(masterAddr, slaverName, slaverToken)
	wait := make(chan int)
	<-wait
}

func sctrlClient() {
	fsck.ShowLog = loglevel
	netw.ShowLog = loglevel > 0
	impl.ShowLog = loglevel > 0
	var err error
	var conf = &WorkConf{}
	var client *fsck.Slaver
	var name = "Sctrl"
	if len(serverAddr) < 1 {
		pwd, _ := os.Getwd()
		conf, err = ReadWorkConf(wsconf)
		if err != nil {
			fmt.Printf("read %v/.sctrl.json fail, %v", pwd, err)
			os.Exit(1)
		}
		serverAddr, loginToken = conf.SrvAddr, conf.Login
		if len(serverAddr) < 1 {
			fmt.Println("server config is empty")
			flag.Usage()
			os.Exit(1)
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
			buf := []byte{}
			for {
				key, err := readkey.ReadKey()
				if err != nil || bytes.Equal(key, CharTerm) {
					readkey.Close()
					os.Exit(1)
				}
				if key[0] == '\r' {
					fmt.Println()
					break
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
			readkey.Close()
			os.Exit(1)
		}
		login <- 1
	}
	webcmd, _ := filepath.Abs(os.Args[0])
	terminal := NewTerminal(client, ps1, bash, webcmd)
	terminal.Name = name
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
		os.Exit(1)
	}
	fmt.Printf("%v is connected\n", serverAddr)
	<-login
	terminal.Proc(conf)
}

func sctrlExec(cmds string) {
	url, _, err := findWebURL("", false, time.Millisecond)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(ExecWebCmd(url+"/exec", cmds, os.Stdout))
}

func sctrlLogCli(name ...string) {
	var url, last string
	var err error
	delay := 5 * time.Second
	for {
		url, last, err = findWebURL(last, true, delay)
		if err != nil { //having error.
			fmt.Println(err)
			os.Exit(1)
		}
		name := strings.Join(name, ",")
		_, err := ExecWebLog(url+"/log", name, os.Stdout)
		if err != nil && reflect.TypeOf(err).String() == "*url.Error" && len(last) > 0 {
			fmt.Printf("->last instance(%v) is offline, will try after %v\n", last, delay)
		} else {
			fmt.Printf("->reply error:%v", err)
		}
		time.Sleep(delay)
	}
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
		if len(webdavUser) < 1 {
			return routing.HRES_CONTINUE
		}
		usr, pwd, ok := hs.R.BasicAuth()
		if !ok {
			hs.W.WriteHeader(403)
			hs.W.Write([]byte("not access\n"))
			return routing.HRES_RETURN
		}
		if fmt.Sprintf("%v:%v", usr, pwd) == webdavUser {
			return routing.HRES_CONTINUE
		}
		hs.W.Write([]byte("not access\n"))
		hs.W.WriteHeader(403)
		return routing.HRES_RETURN
	})
	routing.Shared.Handler("^/dav/.*$", webdav)
	log.Printf("start webdav server by listen(%v),davpth(%v)", webdavAddr, webdavPath)
	err := routing.ListenAndServe(webdavAddr)
	fmt.Println(err)
	os.Exit(1)
}

func findWebURL(last string, wait bool, delay time.Duration) (url string, pwd string, err error) {
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
	for {
		confList := []map[string]interface{}{}
		for _, confPath = range allConf {
			data, err = ioutil.ReadFile(confPath)
			if err == nil {
				break
			}
		}
		if data == nil {
			if wait {
				fmt.Printf("->instance conf is not found on %v, will try after %v\n", allConf, delay)
				time.Sleep(delay)
				continue
			}
			err = fmt.Errorf("find the fsck web url fail")
			return
		}
		err = json.Unmarshal(data, &confList)
		if err != nil {
			err = fmt.Errorf("read fsck config file(%v) fail with %v", confPath, err)
			return
		}
		if len(confList) < 1 {
			//configure not found
			if wait {
				fmt.Printf("->instance list is empty on %v, will try after %v", confPath, delay)
				time.Sleep(delay)
				continue
			}
			err = fmt.Errorf("not running instance")
			return
		}
		if len(last) > 0 {
			//the pwd is specified
			for _, conf := range confList {
				pwd = fmt.Sprintf("%v", conf["pwd"].(string))
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
				fmt.Printf("->last instance(%v) is offline, will try after %v", last, delay)
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
		//create instance info log.
		buf := bytes.NewBuffer(nil)
		for idx, conf := range confList {
			fmt.Fprintf(buf, "%v\t%v\t%v\n", idx, conf["name"], conf["pwd"])
		}
		//
		var rbuf = make([]byte, 1024)
		var key string
		for {
			buf.WriteTo(os.Stdout)
			fmt.Fprintf(os.Stdout, "Please select one(type r to reload)[0]:")
			readed, _ := os.Stdin.Read(rbuf)
			key = string(rbuf[:readed])
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
