

## Install

```.shell
go get github.com/sutils/fsck/sctrl
ln -s $GOPATH/bin/sctrl $GOPATH/bin/sctrl-srv
ln -s $GOPATH/bin/sctrl $GOPATH/bin/sctrl-cli
ln -s $GOPATH/bin/sctrl $GOPATH/bin/sctrl-slaver
ln -s $GOPATH/bin/sctrl $GOPATH/bin/sctrl-log
ln -s $GOPATH/bin/sctrl $GOPATH/bin/sctrl-exec
cp -f $GOPATH/src/github.com/sutils/fsck/sctrl/sctrl-ssh.sh $GOPATH/bin/sctrl-ssh
chmod +x $GOPATH/bin/sctrl-ssh
ln -s $GOPATH/bin/sctrl $GOPATH/bin/sctrl-profile
```