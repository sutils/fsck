#!/bin/bash
if [ $# -lt 1 ];then
    echo "Sctrl ssh version 1.0.0"
    echo "Usage:  sctrl-ssh <name>"
    echo "        sctrl-ssh host1"
    exit 1
fi
cargs=($*)
sargs_=("${cargs[@]:1}")
sargs="${sargs_[@]}"
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
