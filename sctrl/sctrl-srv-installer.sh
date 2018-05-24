#!/bin/bash
case "$1" in
  -i)
    useradd sctrl-srv
    cp -f sctrl-srv.service /etc/systemd/system/
    systemctl enable sctrl-srv.service
    ;;
  *)
    echo "Usage: ./rs-installer.sh (runner|server)"
    ;;
esac