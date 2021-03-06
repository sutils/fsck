#!/bin/bash

if [ $# -lt 2 ];then
    echo "Sctrl scp version 1.0.0"
    echo "Usage:  sctrl-scp <source> <destination>"
    echo "        sctrl-scp ./file1 host1:/home/"
    echo "        sctrl-scp ./dir1 host1:/home/"
    echo "        sctrl-scp host1:/home/file1 /tmp/"
    echo "        sctrl-scp host1:/home/dir1 /tmp/"
    exit 1
fi

path=`dirname ${0}`/sctrl
cmds=`$path -ssh $1`
ecode=$?
if [ "$ecode" == "200" ];then
    eval $cmds
else
    echo $cmds
    echo "exit code:$ecode"
    exit $ecode
fi
