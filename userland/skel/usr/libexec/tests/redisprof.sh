#!/bin/sh
# redis-benchmark (loopback) bracketed by /proc/kstat snapshots.
N=${1:-5000}
C=${2:-8}
redis-server --port 6379 --save '' --appendonly no --loglevel warning > /dev/null &
sleep 2
redis-cli ping > /dev/null
[ -r /proc/kstat ] && { echo "KSTAT-BEGIN"; cat /proc/kstat; echo "KSTAT-END"; }
cat /proc/profmark > /dev/null 2>&1
redis-benchmark -q -n $N -c $C -t get --csv 2>/dev/null | sed 's/^/REDIS /'
[ -r /proc/kstat ] && { echo "KSTAT-BEGIN"; cat /proc/kstat; echo "KSTAT-END"; }
redis-cli shutdown nosave > /dev/null; wait
