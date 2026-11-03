#!/bin/sh
# Redis persistence test: populate, BGSAVE (fork), restart and reload.
mkdir -p /var/lib/redis && cd /var/lib/redis && rm -f dump.rdb
redis-server /etc/redis.conf --loglevel warning &
sleep 1
redis-cli debug populate 100000 key 64
redis-cli dbsize
redis-cli bgsave
while [ "$(redis-cli info persistence | grep rdb_bgsave_in_progress | tr -d '\r')" != "rdb_bgsave_in_progress:0" ]; do sleep 1; done
redis-cli info persistence | grep -E "rdb_last_bgsave_status|rdb_last_cow"
ls -la /var/lib/redis/dump.rdb
redis-cli shutdown nosave; wait
redis-server /etc/redis.conf --loglevel warning &
sleep 3
echo "keys after reload: $(redis-cli dbsize)"
redis-cli get key:4242 | cut -c1-20
redis-cli shutdown nosave; wait
free | grep Mem
