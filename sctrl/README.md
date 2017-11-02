

## Install

```.shell
go get github.com/sutils/fsck/sctrl
fullcmd=$GOPATH/bin/sctrl
alias sctrl=$fullcmd
alias sctrl-server="$fullcmd -s -alias"
alias sctrl-client="$fullcmd -c -alias"
alias sctrl-log="$fullcmd -lc -alias"
alias sctrl-exec="$fullcmd -run -alias"
alias sctrl-ssh="echo 'eval \`$fullcmd -ssh \$1\`' >/tmp/sctrl_ssh.sh && bash -e /tmp/sctrl_ssh.sh"
alias sadd="$fullcmd -run sadd"
alias srm="$fullcmd -run srm"
alias sall="$fullcmd -run sall"
alias spick="$fullcmd -run spick"
alias shelp="$fullcmd -run shelp"
alias sexec="$fullcmd -run sexec"
alias seval="$fullcmd -run seval"
```