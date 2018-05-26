@echo off
cd /d %~dp0
nssm stop "Sctrl Slaver"
nssm remove "Sctrl Slaver" confirm
pause