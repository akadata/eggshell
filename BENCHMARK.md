Benchmarks
==========

eggshell compared with Linux running the same unmodified binaries, on the same
QEMU `microvm` machine, with the same userland.

Setup
-----

|                  | eggshell                                               | Linux                           |
|------------------|--------------------------------------------------------|---------------------------------|
| kernel           | eggshell (this tree, `tamago` go1.27.3)                | Ubuntu 7.0.0-38-generic         |
| root file system | `rootfs/amd64.tar` embedded in the kernel              | `rootfs/amd64.tar` as initramfs |
| host             | 8 vCPU Intel i7-1165Gt @ 2.80GHz                       | same                            |
| machine          | QEMU 11.1.3 `microvm`, 1 vCPU, 2 GiB, `-cpu max` (KVM) | same                            |
| networking       | virtio-net, gVisor TCP/IP                              | virtio-net, Linux TCP/IP        |
| boot arguments   | `quiet ssh=0`                                          | `quiet`                         |

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
| cpu_xorshift_50M | ms | 131.79 | 134.88 | 1.02× faster |
| clock_gettime | ns | 10.21 | 14.06 | 1.38× faster |
| thread_create_join | us | 13.16 | 26.27 | 2.00× faster |
| condvar_pingpong | us | 2.77 | 5.25 | 1.90× faster |
| page_fault | ns/page | 762.33 | 887.02 | 1.16× faster |
| memset_bandwidth | MB/s | 42,433.47 | 40,772.32 | 1.04× faster |
| file_write | MB/s | 8,964.78 | 6,653.44 | 1.35× faster |
| file_write_4k | MB/s | 5,382.87 | 5,268.54 | 1.02× faster |
| file_write_64k | MB/s | 10,609.77 | 7,054.33 | 1.50× faster |
| file_write_1024k | MB/s | 8,722.12 | 6,991.39 | 1.25× faster |
| shell-loop-100k | ms | 157.10 | 169.40 | 1.08× faster |
| rm-go-src | ms | 15.20 | 20.70 | 1.36× faster |
| gunzip-go-binary | ms | 124.90 | 127.80 | 1.02× faster |
| boot to init | ms | 65.50 | 397.70 | 6.07× faster |
| null_syscall | ns | 353.58 | 84.49 | 4.18× slower |
| fork_exit_wait | us | 39.41 | 24.19 | 1.63× slower |
| fork_exec_wait | us | 352.84 | 121.28 | 2.91× slower |
| mmap_touch_munmap | us | 3.13 | 2.86 | 1.09× slower |
| open_close | us | 1.30 | 0.61 | 2.13× slower |
| stat | us | 1.09 | 0.39 | 2.79× slower |
| file_read | MB/s | 10,507.17 | 11,491.84 | 1.09× slower |
| pipe_latency | us | 3.39 | 1.93 | 1.76× slower |
| unix_latency | us | 3.15 | 3.01 | 1.05× slower |
| tcp_latency | us | 15.70 | 5.51 | 2.85× slower |
| pipe_bandwidth | MB/s | 7,914.02 | 10,363.86 | 1.31× slower |
| unix_bandwidth | MB/s | 8,673.21 | 10,691.17 | 1.23× slower |
| tcp_bandwidth | MB/s | 3,674.15 | 8,207.08 | 2.23× slower |
| file_rewrite_4k | MB/s | 5,438.30 | 11,401.17 | 2.10× slower |
| file_read_4k | MB/s | 5,283.22 | 8,653.95 | 1.64× slower |
| file_rewrite_64k | MB/s | 10,499.31 | 13,329.59 | 1.27× slower |
| file_read_64k | MB/s | 10,364.82 | 11,394.62 | 1.10× slower |
| file_rewrite_1024k | MB/s | 9,251.23 | 14,038.24 | 1.52× slower |
| file_read_1024k | MB/s | 7,913.93 | 11,242.26 | 1.42× slower |
| exec-500 | ms | 60.60 | 43.60 | 1.39× slower |
| exec-dynamic-200 | ms | 35.00 | 26.40 | 1.33× slower |
| small-files-2000 | ms | 26.90 | 22.50 | 1.20× slower |
| tar-go-src | ms | 39.90 | 33.70 | 1.18× slower |
| untar-go-src | ms | 92.80 | 65.70 | 1.41× slower |
| gzip-go-binary | ms | 956.40 | 945.90 | 1.01× slower |
| go-build-hello | ms | 12,476.40 | 7,304.50 | 1.71× slower |
| redis SET | req/s | 46,082.95 | 131,578.95 | 2.86× slower |
| redis GET | req/s | 45,454.55 | 128,205.12 | 2.82× slower |
| redis INCR | req/s | 45,454.55 | 125,000.00 | 2.75× slower |
| redis LPUSH | req/s | 45,248.87 | 113,636.37 | 2.51× slower |
| redis LRANGE_100 | req/s | 26,315.79 | 39,525.69 | 1.50× slower |
| redis-p16 SET | req/s | 387,596.91 | 769,230.81 | 1.98× slower |
| redis-p16 GET | req/s | 526,315.81 | 925,925.88 | 1.76× slower |
