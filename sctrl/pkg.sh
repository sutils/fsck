#!/bin/bash
##############################
#####Setting Environments#####
echo "Setting Environments"
set -e
export cpwd=`pwd`
export LD_LIBRARY_PATH=/usr/local/lib:/usr/lib
export PATH=$PATH:$GOPATH/bin:$HOME/bin:$GOROOT/bin
output=build
rm -rf $output
mkdir -p $output

#### Package ####
srv_name=sctrl
srv_ver=1.1.0
srv_out=$output/$srv_name
mkdir -p $srv_out
##build normal
echo "Build $srv_name normal executor..."
go build -o $srv_out/$srv_name github.com/sutils/fsck/sctrl
cp -f .sctrl.json $srv_out
cp -f sctrl-srv-installer.sh $srv_out
cp -f sctrl-srv.service $srv_out
cp -rf example $srv_out

###
cd $output
zip -r $srv_name-$srv_ver-`uname`.zip $srv_name
cd ../
echo "Package $srv_name done..."