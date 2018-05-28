@echo off
cd /d %~dp0
set /p name=name:
set /p token=auth:
mkdir logs
nssm install "Sctrl Slaver" %CD%\sctrl.exe -sc -master rs.dyang.org:9110 -auth %token% -name %name% -cert=certs/server.pem -key=certs/server.key 
nssm set "Sctrl Slaver" AppStdout %CD%\logs\out.log
nssm set "Sctrl Slaver" AppStderr %CD%\logs\err.log
nssm start "Sctrl Slaver"
pause