Benchmarks
==========

eggshell compared with Linux running the same unmodified binaries, on the same
QEMU `microvm` machine, with the same userland.

Setup
-----

|                  | eggshell                                                | Linux                           |
|------------------|---------------------------------------------------------|---------------------------------|
| kernel           | eggshell (this tree, `tamago` go1.27.3)                 | Ubuntu 7.0.0-38-generic         |
| root file system | `rootfs/amd64.tar` embedded in the kernel               | `rootfs/amd64.tar` as initramfs |
| host             | 8 vCPU Intel i7-1165G7 @ 2.80GHz                        | same                            |
| machine          | QEMU 11.1.3 `microvm`, 1 vCPU, 2 GiB, `-cpu host` (KVM) | same                            |
| networking       | virtio-net, gVisor TCP/IP                               | virtio-net, Linux TCP/IP        |
| boot arguments   | `quiet ssh=0`                                           | `quiet`                         |

```sh
# Linux kernel taken from Ubuntu 26.04.1 LTS
./bench/run.py --linux vmlinuz-7.0.0-38-generic --rounds 3 | tee results.md
```

Results
-------

> [!WARNING]
> This project is at its early stages and therefore experimental.

| benchmark (KVM) | unit | eggshell | Linux | eggshell vs Linux |
|---|---|---:|---:|---:|
| cpu_xorshift_50M | ms | 131.97 | 138.02 | 1.05× faster |
| clock_gettime | ns | 10.01 | 15.39 | 1.54× faster |
| thread_create_join | us | 12.72 | 26.62 | 2.09× faster |
| condvar_pingpong | us | 2.69 | 6.36 | 2.36× faster |
| page_fault | ns/page | 775.93 | 877.18 | 1.13× faster |
| file_write | MB/s | 9,421.81 | 6,818.24 | 1.38× faster |
| file_write_64k | MB/s | 10,624.66 | 7,155.46 | 1.48× faster |
| file_write_1024k | MB/s | 8,265.91 | 6,851.58 | 1.21× faster |
| shell-loop-100k | ms | 154.70 | 162.70 | 1.05× faster |
| rm-go-src | ms | 15.00 | 20.80 | 1.39× faster |
| gzip-go-binary | ms | 917.80 | 921.20 | 1.00× faster |
| boot to init | ms | 66.10 | 404.40 | 6.12× faster |
| null_syscall | ns | 353.66 | 84.41 | 4.19× slower |
| fork_exit_wait | us | 37.83 | 23.41 | 1.62× slower |
| fork_exec_wait | us | 347.38 | 117.65 | 2.95× slower |
| memset_bandwidth | MB/s | 40,244.56 | 41,770.61 | 1.04× slower |
| mmap_touch_munmap | us | 3.12 | 2.71 | 1.15× slower |
| open_close | us | 1.30 | 0.58 | 2.24× slower |
| stat | us | 1.05 | 0.37 | 2.84× slower |
| file_read | MB/s | 10,404.25 | 11,581.73 | 1.11× slower |
| pipe_latency | us | 3.56 | 1.84 | 1.93× slower |
| unix_latency | us | 3.17 | 3.12 | 1.02× slower |
| tcp_latency | us | 15.50 | 5.40 | 2.87× slower |
| pipe_bandwidth | MB/s | 8,027.65 | 10,707.96 | 1.33× slower |
| unix_bandwidth | MB/s | 8,882.61 | 10,904.26 | 1.23× slower |
| tcp_bandwidth | MB/s | 3,741.45 | 7,416.24 | 1.98× slower |
| file_write_4k | MB/s | 5,377.33 | 5,405.37 | 1.01× slower |
| file_rewrite_4k | MB/s | 5,363.97 | 11,659.12 | 2.17× slower |
| file_read_4k | MB/s | 5,209.07 | 8,662.77 | 1.66× slower |
| file_rewrite_64k | MB/s | 10,489.42 | 13,736.53 | 1.31× slower |
| file_read_64k | MB/s | 10,341.21 | 11,389.53 | 1.10× slower |
| file_rewrite_1024k | MB/s | 9,435.77 | 13,573.38 | 1.44× slower |
| file_read_1024k | MB/s | 7,903.53 | 11,420.56 | 1.44× slower |
| exec-500 | ms | 61.10 | 42.60 | 1.43× slower |
| exec-dynamic-200 | ms | 34.60 | 26.20 | 1.32× slower |
| small-files-2000 | ms | 26.30 | 22.60 | 1.16× slower |
| tar-go-src | ms | 40.60 | 34.20 | 1.19× slower |
| untar-go-src | ms | 91.90 | 65.60 | 1.40× slower |
| gunzip-go-binary | ms | 124.00 | 122.70 | 1.01× slower |
| go-build-hello | ms | 11,893.40 | 7,274.70 | 1.63× slower |
| redis SET | req/s | 52,110.47 | 123,915.74 | 2.38× slower |
| redis GET | req/s | 52,056.22 | 122,850.12 | 2.36× slower |
| redis INCR | req/s | 51,786.64 | 123,456.79 | 2.38× slower |
| redis LPUSH | req/s | 48,053.82 | 110,132.16 | 2.29× slower |
| redis LRANGE_100 | req/s | 27,382.26 | 39,339.10 | 1.44× slower |
| redis-p16 SET | req/s | 493,339.94 | 747,384.12 | 1.51× slower |
| redis-p16 GET | req/s | 558,347.31 | 931,966.50 | 1.67× slower |
