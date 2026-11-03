/*
 * Kernel microbenchmarks.
 * Output: one "BENCH <name> <value> <unit>" line per test.
 */

#define _GNU_SOURCE
#include <arpa/inet.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/syscall.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

static double now(void)
{
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec + ts.tv_nsec / 1e9;
}

static void report(const char *name, double v, const char *unit)
{
	printf("BENCH %-22s %12.2f %s\n", name, v, unit);
	fflush(stdout);
}

static void bench_syscall(void)
{
	int n = 100000;
	double t = now();

	for (int i = 0; i < n; i++)
		syscall(SYS_getppid);

	report("null_syscall", (now() - t) / n * 1e9, "ns");

	struct timespec ts;
	t = now();

	for (int i = 0; i < n; i++)
		clock_gettime(CLOCK_MONOTONIC, &ts);

	report("clock_gettime", (now() - t) / n * 1e9, "ns");
}

static void bench_fork(void)
{
	int n = 200;
	double t = now();

	for (int i = 0; i < n; i++) {
		pid_t p = fork();
		if (p == 0)
			_exit(0);
		waitpid(p, NULL, 0);
	}

	report("fork_exit_wait", (now() - t) / n * 1e6, "us");

	n = 100;
	t = now();

	for (int i = 0; i < n; i++) {
		pid_t p = fork();
		if (p == 0) {
			execl("/usr/libexec/tests/hello", "hello", NULL);
			_exit(127);
		}
		waitpid(p, NULL, 0);
	}

	report("fork_exec_wait", (now() - t) / n * 1e6, "us");
}

static void *noop(void *a)
{
	return a;
}

static void bench_threads(void)
{
	int n = 500;
	double t = now();

	for (int i = 0; i < n; i++) {
		pthread_t th;
		pthread_create(&th, NULL, noop, NULL);
		pthread_join(th, NULL);
	}

	report("thread_create_join", (now() - t) / n * 1e6, "us");
}

static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t cv = PTHREAD_COND_INITIALIZER;
static int turn;

static void *pingpong_thread(void *a)
{
	for (int i = 0; i < 2000; i++) {
		pthread_mutex_lock(&mu);
		while (turn != 1)
			pthread_cond_wait(&cv, &mu);
		turn = 0;
		pthread_cond_signal(&cv);
		pthread_mutex_unlock(&mu);
	}
	return a;
}

static void bench_condvar(void)
{
	pthread_t th;
	pthread_create(&th, NULL, pingpong_thread, NULL);

	double t = now();

	for (int i = 0; i < 2000; i++) {
		pthread_mutex_lock(&mu);
		turn = 1;
		pthread_cond_signal(&cv);
		while (turn != 0)
			pthread_cond_wait(&cv, &mu);
		pthread_mutex_unlock(&mu);
	}

	report("condvar_pingpong", (now() - t) / 2000 * 1e6, "us");
	pthread_join(th, NULL);
}

static void bench_memory(void)
{
	size_t sz = 64 << 20;
	double t = now();
	char *p = mmap(NULL, sz, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);

	for (size_t i = 0; i < sz; i += 4096)
		p[i] = 1;

	report("page_fault", (now() - t) / (sz / 4096) * 1e9, "ns/page");

	t = now();
	for (int i = 0; i < 10; i++)
		memset(p, i, sz);
	report("memset_bandwidth", 10.0 * sz / (now() - t) / 1e6, "MB/s");
	munmap(p, sz);

	int n = 1000;
	t = now();
	for (int i = 0; i < n; i++) {
		char *q = mmap(NULL, 1 << 20, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
		q[0] = 1;
		munmap(q, 1 << 20);
	}
	report("mmap_touch_munmap", (now() - t) / n * 1e6, "us");
}

static void bench_files(void)
{
	int n = 5000;
	double t = now();

	for (int i = 0; i < n; i++)
		close(open("/etc/passwd", O_RDONLY));

	report("open_close", (now() - t) / n * 1e6, "us");

	struct stat st;
	t = now();
	for (int i = 0; i < n; i++)
		stat("/usr/libexec/tests/bench", &st);
	report("stat", (now() - t) / n * 1e6, "us");

	size_t total = 64 << 20, chunk = 64 << 10;
	char *buf = malloc(chunk);
	memset(buf, 'x', chunk);

	int fd = open("/tmp/bench.dat", O_CREAT | O_TRUNC | O_RDWR, 0644);
	t = now();
	for (size_t off = 0; off < total; off += chunk)
		write(fd, buf, chunk);
	report("file_write", total / (now() - t) / 1e6, "MB/s");

	lseek(fd, 0, SEEK_SET);
	t = now();
	while (read(fd, buf, chunk) > 0)
		;
	report("file_read", total / (now() - t) / 1e6, "MB/s");
	close(fd);
	unlink("/tmp/bench.dat");
	free(buf);
}

static void pingpong(const char *name, int a_rd, int a_wr, int b_rd, int b_wr)
{
	int n = 2000;
	char c = 'x';
	pid_t pid = fork();

	if (pid == 0) {
		for (int i = 0; i < n; i++) {
			if (read(b_rd, &c, 1) != 1)
				_exit(1);
			write(b_wr, &c, 1);
		}
		_exit(0);
	}

	double t = now();

	for (int i = 0; i < n; i++) {
		write(a_wr, &c, 1);
		read(a_rd, &c, 1);
	}

	report(name, (now() - t) / n * 1e6, "us");
	waitpid(pid, NULL, 0);
}

static void stream(const char *name, int rd, int wr)
{
	size_t total = 64 << 20, chunk = 64 << 10;
	pid_t pid = fork();

	if (pid == 0) {
		char *b = malloc(chunk);
		memset(b, 'y', chunk);
		for (size_t s = 0; s < total;) {
			ssize_t w = write(wr, b, chunk);
			if (w <= 0)
				_exit(1);
			s += w;
		}
		_exit(0);
	}

	char *b = malloc(chunk);
	size_t got = 0;
	double t = now();

	while (got < total) {
		ssize_t r = read(rd, b, chunk);
		if (r <= 0)
			break;
		got += r;
	}

	report(name, got / (now() - t) / 1e6, "MB/s");
	waitpid(pid, NULL, 0);
	free(b);
}

static void tcp_pair(int *c, int *s, int port)
{
	int l = socket(AF_INET, SOCK_STREAM, 0), one = 1;
	struct sockaddr_in sa = {.sin_family = AF_INET, .sin_port = htons(port)};
	sa.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
	setsockopt(l, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));
	bind(l, (void *)&sa, sizeof(sa));
	listen(l, 4);
	*c = socket(AF_INET, SOCK_STREAM, 0);
	connect(*c, (void *)&sa, sizeof(sa));
	*s = accept(l, NULL, NULL);
	setsockopt(*c, IPPROTO_TCP, TCP_NODELAY, &one, sizeof(one));
	setsockopt(*s, IPPROTO_TCP, TCP_NODELAY, &one, sizeof(one));
	close(l);
}

static void bench_ipc(void)
{
	int p1[2], p2[2], sv[2], c, s;

	pipe(p1);
	pipe(p2);
	pingpong("pipe_latency", p2[0], p1[1], p1[0], p2[1]);
	close(p1[0]); close(p1[1]); close(p2[0]); close(p2[1]);

	socketpair(AF_UNIX, SOCK_STREAM, 0, sv);
	pingpong("unix_latency", sv[0], sv[0], sv[1], sv[1]);
	close(sv[0]); close(sv[1]);

	tcp_pair(&c, &s, 7001);
	pingpong("tcp_latency", c, c, s, s);
	close(c); close(s);

	pipe(p1);
	stream("pipe_bandwidth", p1[0], p1[1]);
	close(p1[0]); close(p1[1]);

	socketpair(AF_UNIX, SOCK_STREAM, 0, sv);
	stream("unix_bandwidth", sv[0], sv[1]);
	close(sv[0]); close(sv[1]);

	tcp_pair(&c, &s, 7002);
	stream("tcp_bandwidth", s, c);
	close(c); close(s);
}

static void bench_cpu(void)
{
	/* pure user mode computation, independent of the kernel */
	double t = now();
	uint64_t x = 88172645463325252ULL, acc = 0;

	for (long i = 0; i < 50000000; i++) {
		x ^= x << 13;
		x ^= x >> 7;
		x ^= x << 17;
		acc += x % 1000003;
	}

	report("cpu_xorshift_50M", (now() - t) * 1e3, "ms");

	if (acc == 42)
		printf("%llu\n", (unsigned long long)acc);
}

int main(int argc, char **argv)
{
	setvbuf(stdout, NULL, _IOLBF, 0);

	bench_cpu();
	bench_syscall();
	bench_fork();
	bench_threads();
	bench_condvar();
	bench_memory();
	bench_files();
	bench_ipc();

	printf("BENCH done\n");

	return 0;
}
