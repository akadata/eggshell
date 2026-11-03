#!/bin/bash
# Shell level smoke test: process creation, pipes, signals, files, redis.
echo "== fork/exec rate"
time (for i in $(seq 200); do /bin/true; done)

echo "== shell features"
trap 'echo trapped USR1' USR1; kill -USR1 $$
f() { local x=$1; echo $((x*2)); }; echo "f=$(f 21)"
find /etc -name '*.conf' | sort
grep -c root /etc/passwd
echo "sha: $(echo hello | sha256sum)"
seq 1 100000 | awk '{s+=$1} END {print "awk sum", s}'

echo "== file I/O"
dd if=/dev/zero of=/tmp/big bs=1M count=50 2>&1 | tail -1
ls -la /tmp/big; md5sum /tmp/big; rm /tmp/big
df -h / | tail -1

echo "== redis"
redis-server --port 6379 --save '' --loglevel warning > /dev/null &
sleep 1
redis-benchmark -q -n 5000 -c 8 -t set,get,incr,lpush,lrange_100
redis-cli shutdown nosave; wait

echo "== proc"
head -4 /proc/self/status
uptime
