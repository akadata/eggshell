#!/bin/sh
# BusyBox applets smoke test.
set -e
cd /tmp
mkdir -p tree/a/b && echo one > tree/a/1.txt && echo two > tree/a/b/2.txt
tar czf tree.tgz tree && rm -rf tree && tar xzf tree.tgz && cat tree/a/b/2.txt
find tree -name '*.txt' -exec cat {} \; | sort | xargs echo xargs:
echo "hello world" | sed 's/world/sed/' | tr a-z A-Z
printf '3\n1\n2\n' | sort -n | tail -1
seq 10 | paste -sd+ | bc 2>/dev/null || echo "sum: $(( $(seq 10 | paste -sd+) ))"
gzip -c /bin/busybox | wc -c > /tmp/gz.size && echo "gzip ok $(cat /tmp/gz.size)"
timeout 1 sleep 5 || echo "timeout rc=$?"
date -u +%Y > /dev/null && echo date ok
(echo GET / | nc -l -p 9999 > /tmp/nc.out &) ; sleep 1
echo "via nc" | nc 127.0.0.1 9999 || true
sleep 1; cat /tmp/nc.out
mkdir -p /www && echo '<h1>Hello from httpd</h1>' > /www/index.html
httpd -h /www -p 8080 && echo httpd started
wget -q -O - http://127.0.0.1:8080/
id; hostname; uname -m
echo ALL-BUSYBOX-OK
