#!/bin/bash
go get github.com/sutils/fsck/sctrl
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-srv
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-cli
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-slaver
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-log
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-profile
ln -sf $GOPATH/bin/sctrl $GOPATH/bin/sctrl-shell
ln -sf $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-ssh.sh $GOPATH/bin/sctrl-ssh
ln -sf $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-scp.sh $GOPATH/bin/sctrl-scp
ln -sf $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-ws.sh $GOPATH/bin/sctrl-ws
chmod +x $GOPATH/bin/sctrl-ssh $GOPATH/bin/sctrl-scp $GOPATH/bin/sctrl-scp
echo "all done..."