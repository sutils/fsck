@echo off
set srv_name=sctrl
set srv_ver=1.3.4
del /s /a /q build\%srv_name%
mkdir build
mkdir build\%srv_name%
go build -o build\%srv_name%\sctrl.exe github.com/sutils/fsck/sctrl
if NOT %ERRORLEVEL% EQU 0 goto :efail
reg Query "HKLM\Hardware\Description\System\CentralProcessor\0" | find /i "x86" > NUL && set OS=x86||set OS=x64
xcopy win-%OS%\nssm.exe build\%srv_name%
xcopy sctrl-conf.bat build\%srv_name%
xcopy sctrl-install.bat build\%srv_name%
xcopy sctrl-uninstall.bat build\%srv_name%
mkdir build\%srv_name%\example
xcopy /s /e /h example build\%srv_name%\example\
mkdir build\%srv_name%\certs
echo "make server cert"
openssl req -new -nodes -x509 -out build\%srv_name%\certs\server.pem -keyout build\%srv_name%\certs\server.key -days 3650 -subj "/C=CN/ST=NRW/L=Earth/O=Random Company/OU=IT/CN=rsck.dyang.org/emailAddress=cert@dyang.org"
echo "make slaver cert"
openssl req -new -nodes -x509 -out build\%srv_name%\certs\slaver.pem -keyout build\%srv_name%\certs\slaver.key -days 3650 -subj "/C=CN/ST=NRW/L=Earth/O=Random Company/OU=IT/CN=rsck.dyang.org/emailAddress=cert@dyang.org"
echo "make client cert"
openssl req -new -nodes -x509 -out build\%srv_name%\certs\client.pem -keyout build\%srv_name%\certs\client.key -days 3650 -subj "/C=CN/ST=NRW/L=Earth/O=Random Company/OU=IT/CN=rsck.dyang.org/emailAddress=cert@dyang.org"
if NOT %ERRORLEVEL% EQU 0 goto :efail

cd build
del /s /a /q %srv_name%-%srv_ver%-Win-%OS%.zip
7z a -r %srv_name%-%srv_ver%-Win-%OS%.zip %srv_name%
if NOT %ERRORLEVEL% EQU 0 goto :efail
cd ..\
goto :esuccess

:efail
echo "Build fail"
pause
exit 1

:esuccess
echo "Build success"
pause