#!/bin/bash
set -e
rm -f test/test.zip
rm -f /tmp/test.zip
rm -f /tmp/testa.zip
#
echo testing $1
zip -r /tmp/test.zip .
echo upload
curl -T /tmp/test.zip $1 >/dev/null
echo download
curl -o /tmp/testa.zip $1 >/dev/null
cd /tmp/
unzip testa.zip
echo "all done"