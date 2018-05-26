@echo off
cd /d %~dp0
set /p name=���������ƣ�
set /p token=���������룺
mkdir logs
nssm install "Sctrl Slaver" %CD%\sctrl.exe -sc -master rs.dyang.org:9121 -auth %token% -name %name% -showlog=1 -cert=certs/server.pem -key=certs/server.key 
nssm set "Sctrl Slaver" AppStdout %CD%\logs\out.log
nssm set "Sctrl Slaver" AppStderr %CD%\logs\err.log
nssm start "Sctrl Slaver"
pause