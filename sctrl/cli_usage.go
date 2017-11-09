package main

import "fmt"

type Usage string

func NewUsage(format string, args ...interface{}) *Usage {
	str := Usage(fmt.Sprintf(format, args...))
	return &str
}

func (u *Usage) Append(format string, args ...interface{}) *Usage {
	*u = Usage(string(*u) + fmt.Sprintf(format, args...))
	return u
}

func (u *Usage) String() string {
	return string(*u)
}

func (u *Usage) Error() string {
	return string(*u)
}

var sallUsage = NewUsage("Sctrl sall version %v\n", Version).
	Append("       sall will show all picked host info\n").
	Append("       using spick to activate and unactivate host\n").
	Append("Usage: sall\n")

var saddUsage = NewUsage("Sctrl sadd version %v\n", Version).
	Append("       add remote host by name and uri\n").
	Append("Usage: sadd <name> <uri> [connect]\n").
	Append("       sadd host1 master://localhost connect\n").
	Append("       sadd host2 test://192.168.1.10\n").
	Append("Options:\n").
	Append("  name\n").
	Append("       the remote host alias\n").
	Append("  uri\n").
	Append("       the remote host uri by channel://host:port\n").
	Append("  connect\n").
	Append("       whether connect host immediately\n")

var srmUsage = NewUsage("Sctrl srm version %v\n", Version).
	Append("       remove remote host by name\n").
	Append("Usage: srm <name>\n").
	Append("       srm host1\n").
	Append("Options:\n").
	Append("  name\n").
	Append("       the remote host alias\n")

var spickUsage = NewUsage("Sctrl spick version %v\n", Version).
	Append("       activate and unactivate remote host by name list\n").
	Append("       using all as arguemnt will activate all host\n").
	Append("Usage: spick <name> <name2>\n").
	Append("       spick host1 host2\n").
	Append("       spick all\n").
	Append("Options:\n").
	Append("  name\n").
	Append("       the remote host alias\n")

var sexeclUsage = NewUsage("Sctrl sexec version %v\n", Version).
	Append("       sexec execute the command on picked remote host\n").
	Append("       using spick to activate and unactivate host\n").
	Append("Usage: sexec <cmd> <arg1> <arg2> <...>\n").
	Append("       sexec echo arg1 arg2\n").
	Append("       sexec uptime\n")

var sevalUsage = NewUsage("Sctrl seval version %v\n", Version).
	Append("       seval will upload local script file to picked remote host and execute by ./script args\n").
	Append("       using spick to activate and unactivate host\n").
	Append("Usage: seval <script file> <arg1> <arg2> <...>\n").
	Append("       seval ./show.sh arg1 arg2\n")

var saddmapUsage = NewUsage("Sctrl saddmap version %v\n", Version).
	Append("       saddmap will bind local address to remote host by uri\n").
	Append("Usage: saddmap <name> [<local>] <remote>\n").
	Append("       saddmap rsync :2832 master://localhost:223\n").
	Append("       saddmap rsync2 :2832 test1://192.168.1.100:223\n").
	Append("       saddmap rsync3 test2://192.168.1.100:223\n").
	Append("Options:\n").
	Append("  name\n").
	Append("       the forward alias\n").
	Append("  local\n").
	Append("       the local address to listen, it will be like :2322 or 127.0.0.1:2322\n").
	Append("       if it is not setted, will auto select one\n").
	Append("  remote\n").
	Append("       the remote host uri to connect, it will be like channel://host:port\n").
	Append("       eg: master://localhost:232,  test1://192.168.1.100:232\n")

var srmmapUsage = NewUsage("Sctrl srmmap version %v\n", Version).
	Append("       srmmap will remove binded local address to remote host by name\n").
	Append("Usage: srmmap <name>\n").
	Append("       srmmap rsync\n").
	Append("Options:\n").
	Append("  name\n").
	Append("       the forward alias\n")

var slsmapUsage = NewUsage("Sctrl slsmap version %v\n", Version).
	Append("       slsmap will show all forward info\n").
	Append("Usage: slsmap [<name>]\n").
	Append("       slsmap rsync\n").
	Append("Options:\n").
	Append("  name\n").
	Append("       the forward alias\n")

var smasterUsage = NewUsage("Sctrl smaster version %v\n", Version).
	Append("       smaster will show the master status\n").
	Append("Usage: smaster\n")

var sslaverUsage = NewUsage("Sctrl sslaver version %v\n", Version).
	Append("       sslaver will show the slaver status\n").
	Append("Usage: sslaver <name> <name1>\n").
	Append("       sslaver test\n")

var spingUsage = NewUsage("Sctrl ping version %v\n", Version).
	Append("       sping will ping to remote slaver and return the delay\n").
	Append("Usage: sping <host name>\n").
	Append("       sping host1\n")

var shelpUsage = NewUsage("Sctrl version %v\n", Version).
	Append("       sctrl console is helpful tool to manager multi host in inner network.\n").
	Append("Supported:\n").
	Append("\n%v\n", sallUsage).
	Append("\n%v\n", saddUsage).
	Append("\n%v\n", srmUsage).
	Append("\n%v\n", spickUsage).
	Append("\n%v\n", sexeclUsage).
	Append("\n%v\n", sevalUsage).
	Append("\n%v\n", saddmapUsage).
	Append("\n%v\n", srmmapUsage).
	Append("\n%v\n", smasterUsage)
