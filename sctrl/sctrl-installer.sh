#!/bin/bash
case "$1" in
  sctrl-srv)
    useradd sctrl-srv
    cp -f sctrl-srv.service /etc/systemd/system/
    systemctl enable sctrl-srv.service
    ;;
  sctrl-sc)
    useradd sctrl-sc
    cp -f sctrl-sc.service /etc/systemd/system/
    systemctl enable sctrl-sc.service
    ;;
  *)
    echo "Usage: ./sctrl-installer.sh (sctrl-srv|sctrl-sc)"
    ;;
esac