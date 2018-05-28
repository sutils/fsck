

### Install

```.shell
go get github.com/sutils/fsck/sctrl
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-srv
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-cli
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-slaver
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-log
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-exec
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-sreal
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-profile
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-shell
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-put
ln -sf $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-ssh.sh $GOPATH/bin/sctrl-ssh
ln -sf $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-scp.sh $GOPATH/bin/sctrl-scp
ln -sf $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-ws.sh $GOPATH/bin/sctrl-ws
chmod +x $GOPATH/bin/sctrl-ssh $GOPATH/bin/sctrl-scp $GOPATH/bin/sctrl-ws
```

### Features

* port forwad by
  * `user->sctrl master->server`
  * `user->sctrl master->sctrl slaver->server`
  * `user->sctrl client->sctrl master->server`
  * `user->sctrl client->sctrl master->sctrl slaver->server`
* connect keeping，auto reconenct when client->master/master->slaver is breaken
* multi bash utils

### Foward


### Usage

##### sctrl-srv
run the sctrl master server, it is a alias by `sctrl -s`

* example
  * basic:`sctrl-srv -listen :9121 -token abc=1 -cert certs/server.pem -key certs/server.key`
  * having webuid:`sctrl-srv -listen :9121 -token abc=1 -cert certs/server.pem -key certs/server.key -webaddr :9090`
  * host forward:`sctrl-srv -listen :9121 -token abc=1 -cert certs/server.pem -key certs/server.key -webaddr :9090 -websuffix .xx.xxx.com`
* the systemctl service config is `sctrl-srv.service`
* list all arguments by `sctrl-srv -h`

##### sctrl-slaver
run the sctrl slaver client, it is a alias by `sctrl -sc`

* example
  * basic:`sctrl-slaver -master localhost:9234 -auth abc -name test -cert=certs/server.pem -key=certs/server.key`
* the systemctl service config is `sctrl-sc.service`
* list all arguments by `sctrl-slaver -h`

##### sctrl-client
run the sctrl client, it is a alias by `sctrl -c`

* example
  * basic:`sctrl-cli -webaddr :9091 -cert certs/client.pem -key certs/client.key -server localhost:9234`
  * using config(`.sctrl.json`):`sctrl-cli -webaddr :9091 -cert certs/client.pem -key certs/client.key`
* list all arguments by `sctrl-slaver -h`
* the webui to show/add forward: `http://localhost:9091`
* the example config file

```.json
{
    "name": "Sctrl",
    "server": "localhost:9234",
    "login": "",
    "bash": "/bin/bash",
    "ps1": "Sctrl \\W>",
    "instance": "/tmp/.sctrl_instance.json",
    "hosts": [
        {
            "name": "loc",
            "uri": "test://root:sco@loc.m?pty=vt100",
            "startup": 0,
            "env": {
                "name1": "value2",
                "name2": "value3"
            }
        }
    ],
    "forward": {
        "loc": "tcp://:2943<test>tcp://loc.m:80",
        "bash": "tcp://:2940<test>tcp://cmd?exec=bash",
        "ws1": "ws://bash<test>tcp://cmd?exec=bash",
        "ws2": "ws://t0<test>tcp://cmd?exec=bash",
        "win": "tcp://:1232<test>tcp://10.211.55.31:3389"
    },
    "env": {
        "name1": "value1"
    }
}
```

##### sctrl-log
show the session log, it is a alias by `sctrl -lc`

* example: `sctrl-log loc`
* list all arguments by `sctrl-log -h`

##### sctrl-exec
run sctrl command, it is a alias by `sctrl -run`

* example: `sctrl-exec sadd host root:xxx@host.local`
* list all arguments by `sctrl-exec shelp`

##### sctrl-ssh
start ssh by sctrl forward

* external tools required:`sshpass`
* example
  * ssh command line: `sctrl-ssh bash`
  * ssh remote command: `sctrl-ssh bash <commond>`

##### sctrl-scp
start scp by sctrl forward

* external tools required:`sshpass`
* example:`sctrl-scp loc:/home/file1 /tmp/`

##### sctrl-shell
start remote command on slaver and forward stdin/stdout to local by sctrl exec forward

* for the remote base example, it can be forward by blow
  * when forward by `ws://name<slaver name>tcp://cmd?exec=bash`, connect by `sctrl-shell name` or `sctrl-shell ws://name.x.xxx.com:xxx` or `sctrl-shell ws://x.xxx.com:xxx/ws/name`
  * when forward by `tcp://127.0.0.1:xxx<slaver name>tcp://cmd?exec=bash`, connect by `sctrl-shell tcp://localhost:xxx`

##### webdav forward
start webdav server on slaver by sctrl forward

* port forward to slaver by `tcp://:2232<slaver name>http://web?dir=/tmp/`
  * list by `wget http://xx.xxx.com:2232` or browser to `http://xx.xxx.com:2232`
  * upload file by `sctrl-put <file> http://x.xxx.com:2232/xx/` or `curl -X`
* port forward by slaver by `web://name<slaver name>http://web?dir=/tmp/`
  * list by `wget http://name.xx.xxx.com:xxx` or browser to `http://xx.xxx.com:2232`
  * upload file by `sctrl-put <file> http://name.x.xxx.com:xxx/xx/` or `curl -X`