#!/bin/bash
cargs=($*)
sargs_=("${cargs[@]:1}")
sargs="${sargs_[@]}"
path=`dirname ${0}`/sctrl
eval `$path -ssh $1`