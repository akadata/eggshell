#!/bin/sh
# In-guest redis benchmark (loopback TCP), N sets the number of requests
# (enough for runs of seconds, as redis-benchmark measures in milliseconds).
redis-server --port 6379 --save '' --appendonly no --loglevel warning > /dev/null &
sleep 2
redis-benchmark --csv -q -n ${N:-100000} -c 8 -t set,get,incr,lpush,lrange_100 2>/dev/null | sed 's/^/REDIS /'
redis-benchmark --csv -q -n $((${N:-100000} * 10)) -c 8 -P 16 -t set,get 2>/dev/null | sed 's/^/REDISP16 /'
redis-cli shutdown nosave > /dev/null; wait
echo REDIS done
