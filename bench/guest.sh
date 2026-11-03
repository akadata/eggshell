#!/bin/sh
#
# In-guest benchmark suite, run unmodified on eggshell and Linux (see run.py).
export PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin HOME=/root
B=/root/bench
t() { n=$1; shift; $B/tm "$n" "$@"; }

cd /tmp
echo "SUITE-START"

# micro benchmarks
/usr/libexec/tests/bench | grep -v 'BENCH done'
$B/fio

# processes
t exec-500 sh -c 'i=0; while [ $i -lt 500 ]; do /bin/true; i=$((i+1)); done'
t exec-dynamic-200 sh -c 'i=0; while [ $i -lt 200 ]; do /usr/libexec/tests/hello-dynamic; i=$((i+1)); done'
t shell-loop-100k sh -c 'i=0; while [ $i -lt 100000 ]; do i=$((i+1)); done'

# file system
t small-files-2000 sh -c 'mkdir s && cd s && i=0; while [ $i -lt 2000 ]; do echo x > $i; i=$((i+1)); done; cat * > /dev/null; cd .. && rm -r s'
t tar-go-src tar -C /usr/local/go -cf /tmp/src.tar src
t untar-go-src sh -c 'mkdir x && tar -C x -xf src.tar'
t rm-go-src rm -rf x src.tar

# compute and I/O
t gzip-go-binary sh -c 'gzip -c /usr/local/go/bin/go > go.gz'
t gunzip-go-binary sh -c 'gzip -dc go.gz > /dev/null && rm go.gz'
t go-build-hello sh $B/gobuild.sh

# loopback TCP server
sh /usr/libexec/tests/redisbench.sh

echo "SUITE-END"
