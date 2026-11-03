/*
 * POSIX conformance smoke test for TamaGo POSIX unikernels.
 *
 * Exercises process, signal, thread, memory, file system, I/O multiplexing
 * and time interfaces, printing one line per check and a final summary.
 */

#define _GNU_SOURCE
#include <dirent.h>
#include <errno.h>
#include <fenv.h>
#include <fcntl.h>
#include <math.h>
#include <netinet/in.h>
#include <poll.h>
#include <pthread.h>
#include <setjmp.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/epoll.h>
#include <sys/mman.h>
#include <sys/select.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/time.h>
#include <sys/un.h>
#include <sys/auxv.h>
#include <sys/syscall.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

static volatile int passed, failed;

#define CHECK(name, cond) do { \
	if (cond) { passed++; printf("ok   %s\n", name); } \
	else { failed++; printf("FAIL %s (errno %d: %s)\n", name, errno, strerror(errno)); } \
	fflush(stdout); \
} while (0)

static void test_fork_wait(void)
{
	int status;
	pid_t pid = fork();

	if (pid == 0)
		_exit(42);

	CHECK("fork", pid > 0);
	CHECK("waitpid exit status", waitpid(pid, &status, 0) == pid && WIFEXITED(status) && WEXITSTATUS(status) == 42);

	pid = fork();

	if (pid == 0) {
		pause();
		_exit(0);
	}

	kill(pid, SIGTERM);
	CHECK("waitpid signaled", waitpid(pid, &status, 0) == pid && WIFSIGNALED(status) && WTERMSIG(status) == SIGTERM);
	CHECK("ECHILD", waitpid(-1, &status, WNOHANG) == -1 && errno == ECHILD);
}

static void test_pipe_exec(void)
{
	int fd[2];
	char buf[128] = {0};

	CHECK("pipe", pipe(fd) == 0);

	pid_t pid = fork();

	if (pid == 0) {
		dup2(fd[1], 1);
		close(fd[0]);
		close(fd[1]);
		execl("/bin/echo", "echo", "exec-ok", NULL);
		_exit(127);
	}

	close(fd[1]);

	int n = 0, r;

	while ((r = read(fd[0], buf + n, sizeof(buf) - 1 - n)) > 0)
		n += r;

	close(fd[0]);
	waitpid(pid, NULL, 0);
	CHECK("fork+exec+pipe", strcmp(buf, "exec-ok\n") == 0);

	FILE *p = popen("echo popen-ok | tr a-z A-Z", "r");
	memset(buf, 0, sizeof(buf));
	fgets(buf, sizeof(buf), p);
	CHECK("popen", pclose(p) == 0 && strcmp(buf, "POPEN-OK\n") == 0);

	CHECK("system", system("exit 3") >> 8 == 3);
}

static volatile sig_atomic_t got;

static void handler(int sig, siginfo_t *si, void *uc)
{
	got = sig;
}

static jmp_buf jb;

/* opaque invalid pointer: a constant one is undefined behaviour the compiler
 * may reorder around */
static volatile int *volatile badptr = (volatile int *)16;

static void segv_handler(int sig)
{
	siglongjmp(jb, 1);
}

static void test_signals(void)
{
	struct sigaction sa = {0};

	sa.sa_sigaction = handler;
	sa.sa_flags = SA_SIGINFO;
	sigaction(SIGUSR1, &sa, NULL);

	got = 0;
	raise(SIGUSR1);
	CHECK("raise+handler", got == SIGUSR1);

	sigset_t set, old;
	sigemptyset(&set);
	sigaddset(&set, SIGUSR1);
	sigprocmask(SIG_BLOCK, &set, &old);
	got = 0;
	kill(getpid(), SIGUSR1);
	CHECK("blocked signal pending", got == 0);
	sigpending(&set);
	CHECK("sigpending", sigismember(&set, SIGUSR1));
	sigprocmask(SIG_SETMASK, &old, NULL);
	CHECK("unblocked delivery", got == SIGUSR1);

	got = 0;
	sigaction(SIGALRM, &sa, NULL);
	alarm(1);
	pause();
	CHECK("alarm+pause", got == SIGALRM);

	signal(SIGSEGV, segv_handler);

	if (sigsetjmp(jb, 1) == 0) {
		*badptr = 1;
		CHECK("SIGSEGV handler", 0);
	} else {
		CHECK("SIGSEGV handler", 1);
	}

	/* EINTR on blocking read */
	int fd[2];
	pipe(fd);
	sa.sa_flags = SA_SIGINFO;
	sigaction(SIGALRM, &sa, NULL);
	struct itimerval it = {{0, 0}, {0, 100000}};
	setitimer(ITIMER_REAL, &it, NULL);
	char c;
	errno = 0;
	CHECK("read EINTR", read(fd[0], &c, 1) == -1 && errno == EINTR);
	close(fd[0]);
	close(fd[1]);

	signal(SIGPIPE, SIG_IGN);
	pipe(fd);
	close(fd[0]);
	CHECK("EPIPE", write(fd[1], "x", 1) == -1 && errno == EPIPE);
	close(fd[1]);
}

static pthread_mutex_t mu = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t cv = PTHREAD_COND_INITIALIZER;
static int counter, ready;

static void *worker(void *arg)
{
	for (int i = 0; i < 10000; i++) {
		pthread_mutex_lock(&mu);
		counter++;
		pthread_mutex_unlock(&mu);
	}

	pthread_mutex_lock(&mu);
	ready++;
	pthread_cond_broadcast(&cv);
	pthread_mutex_unlock(&mu);

	return (void *)(long)gettid();
}

static __thread int tls = 7;

static void *tls_worker(void *arg)
{
	tls = 99;
	return (void *)(long)tls;
}

static void test_threads(void)
{
	pthread_t th[4];

	for (int i = 0; i < 4; i++)
		pthread_create(&th[i], NULL, worker, NULL);

	pthread_mutex_lock(&mu);
	while (ready < 4)
		pthread_cond_wait(&cv, &mu);
	pthread_mutex_unlock(&mu);

	for (int i = 0; i < 4; i++)
		pthread_join(th[i], NULL);

	CHECK("pthread mutex/cond", counter == 40000);

	void *r;
	pthread_create(&th[0], NULL, tls_worker, NULL);
	pthread_join(th[0], &r);
	CHECK("thread local storage", (long)r == 99 && tls == 7);
}

static void test_memory(void)
{
	size_t sz = 64 << 20;
	char *p = mmap(NULL, sz, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);

	CHECK("mmap 64MB", p != MAP_FAILED);
	p[0] = 1;
	p[sz - 1] = 2;
	CHECK("demand paging", p[0] == 1 && p[sz - 1] == 2 && p[sz / 2] == 0);
	CHECK("munmap", munmap(p, sz) == 0);

	char *m = malloc(10 << 20);
	memset(m, 0x5a, 10 << 20);
	m = realloc(m, 20 << 20);
	CHECK("malloc/realloc", m && m[(10 << 20) - 1] == 0x5a);
	free(m);

	int *shared = mmap(NULL, 4096, PROT_READ | PROT_WRITE, MAP_SHARED | MAP_ANONYMOUS, -1, 0);
	*shared = 0;

	if (fork() == 0) {
		*shared = 1234;
		_exit(0);
	}

	wait(NULL);
	CHECK("MAP_SHARED across fork", *shared == 1234);

	int priv = 5;

	if (fork() == 0) {
		priv = 6;
		_exit(priv);
	}

	int status;
	wait(&status);
	CHECK("fork copy isolation", priv == 5 && WEXITSTATUS(status) == 6);

	char *ro = mmap(NULL, 4096, PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0);
	ro[0] = 'a';
	mprotect(ro, 4096, PROT_READ);
	signal(SIGSEGV, segv_handler);

	if (sigsetjmp(jb, 1) == 0) {
		// volatile, so that the faulting store is not moved past CHECK
		*(volatile char *)ro = 'b';
		CHECK("mprotect write fault", 0);
	} else {
		CHECK("mprotect write fault", ro[0] == 'a');
	}
}

static void test_files(void)
{
	mkdir("/tmp/ptest", 0755);
	CHECK("chdir", chdir("/tmp/ptest") == 0);

	char cwd[256];
	CHECK("getcwd", getcwd(cwd, sizeof(cwd)) && strcmp(cwd, "/tmp/ptest") == 0);

	int fd = open("f.txt", O_CREAT | O_RDWR | O_TRUNC, 0644);
	CHECK("open O_CREAT", fd >= 0);
	CHECK("write", write(fd, "hello world", 11) == 11);
	CHECK("lseek", lseek(fd, 6, SEEK_SET) == 6);

	char buf[32] = {0};
	CHECK("read", read(fd, buf, sizeof(buf)) == 5 && strcmp(buf, "world") == 0);

	struct stat st;
	CHECK("fstat", fstat(fd, &st) == 0 && st.st_size == 11 && S_ISREG(st.st_mode));
	CHECK("pwrite/pread", pwrite(fd, "W", 1, 6) == 1 && pread(fd, buf, 5, 6) == 5 && memcmp(buf, "World", 5) == 0);
	CHECK("ftruncate", ftruncate(fd, 5) == 0 && fstat(fd, &st) == 0 && st.st_size == 5);

	char *map = mmap(NULL, 5, PROT_READ, MAP_PRIVATE, fd, 0);
	CHECK("mmap file", map != MAP_FAILED && memcmp(map, "hello", 5) == 0);
	close(fd);

	CHECK("rename", rename("f.txt", "g.txt") == 0 && access("g.txt", F_OK) == 0 && access("f.txt", F_OK) == -1);
	CHECK("link", link("g.txt", "h.txt") == 0 && stat("h.txt", &st) == 0 && st.st_nlink == 2);
	CHECK("symlink/readlink", symlink("g.txt", "s") == 0 && readlink("s", buf, sizeof(buf)) == 5);
	CHECK("lstat", lstat("s", &st) == 0 && S_ISLNK(st.st_mode));
	CHECK("mkdir", mkdir("d", 0700) == 0 && stat("d", &st) == 0 && S_ISDIR(st.st_mode));

	DIR *d = opendir(".");
	int entries = 0;
	struct dirent *e;

	while ((e = readdir(d)))
		entries++;

	closedir(d);
	CHECK("readdir", entries == 6);

	CHECK("rmdir non-empty", mkdir("d/x", 0700) == 0 && rmdir("d") == -1 && errno == ENOTEMPTY);
	CHECK("unlink", unlink("s") == 0 && unlink("h.txt") == 0 && unlink("g.txt") == 0);
	CHECK("ENOENT", open("nope", O_RDONLY) == -1 && errno == ENOENT);

	FILE *f = fopen("/tmp/ptest/stdio.txt", "w+");
	fprintf(f, "%d %s %.3f\n", 42, "str", 3.14159);
	rewind(f);
	int i;
	char s[8];
	double x;
	CHECK("stdio", fscanf(f, "%d %7s %lf", &i, s, &x) == 3 && i == 42 && fabs(x - 3.142) < 1e-9);
	fclose(f);

	int fd2 = dup(1);
	CHECK("dup/fcntl", fd2 > 2 && fcntl(fd2, F_SETFD, FD_CLOEXEC) == 0 && fcntl(fd2, F_GETFD) == FD_CLOEXEC);
	close(fd2);
	CHECK("isatty(/dev/null)", (fd = open("/dev/null", O_RDWR)) >= 0 && !isatty(fd));
	close(fd);
	chdir("/");
}

static void test_poll(void)
{
	int fd[2];
	pipe(fd);

	struct pollfd pfd = {fd[0], POLLIN, 0};
	CHECK("poll timeout", poll(&pfd, 1, 50) == 0);
	write(fd[1], "x", 1);
	CHECK("poll readable", poll(&pfd, 1, -1) == 1 && (pfd.revents & POLLIN));

	fd_set rs;
	FD_ZERO(&rs);
	FD_SET(fd[0], &rs);
	struct timeval tv = {0, 1000};
	CHECK("select", select(fd[0] + 1, &rs, NULL, NULL, &tv) == 1 && FD_ISSET(fd[0], &rs));

	int ep = epoll_create1(0);
	struct epoll_event ev = {.events = EPOLLIN, .data.u64 = 0xdeadbeef}, out;
	epoll_ctl(ep, EPOLL_CTL_ADD, fd[0], &ev);
	CHECK("epoll", epoll_wait(ep, &out, 1, 100) == 1 && out.data.u64 == 0xdeadbeef);
	char c;
	read(fd[0], &c, 1);
	CHECK("epoll timeout", epoll_wait(ep, &out, 1, 20) == 0);
	close(ep);

	fcntl(fd[0], F_SETFL, O_NONBLOCK);
	CHECK("O_NONBLOCK EAGAIN", read(fd[0], &c, 1) == -1 && errno == EAGAIN);
	close(fd[0]);
	close(fd[1]);

	int sv[2];
	CHECK("socketpair", socketpair(AF_UNIX, SOCK_STREAM, 0, sv) == 0);
	write(sv[0], "ping", 4);
	char b[8] = {0};
	CHECK("socketpair io", read(sv[1], b, 4) == 4 && strcmp(b, "ping") == 0);
	close(sv[0]);
	close(sv[1]);
}

// An accepted connection closed by the peer may be released by the network
// stack before the server closes its socket: a later connection must not be
// affected by the stale socket (its data, its close).
static void test_tcp_reuse(void)
{
	struct sockaddr_in sin = {.sin_family = AF_INET, .sin_addr.s_addr = htonl(INADDR_LOOPBACK)};
	socklen_t len = sizeof(sin);
	struct timeval tv = {.tv_sec = 2};
	const char *step = NULL;
	int i, ls = socket(AF_INET, SOCK_STREAM, 0);

#define STEP(cond, name) do { if (!step && !(cond)) step = name; } while (0)
	STEP(ls >= 0 && bind(ls, (struct sockaddr *)&sin, len) == 0 &&
		getsockname(ls, (struct sockaddr *)&sin, &len) == 0 && listen(ls, 8) == 0, "listen");

	for (i = 0; i < 10 && !step; i++) {
		char c = 0;
		int a = socket(AF_INET, SOCK_STREAM, 0);
		STEP(connect(a, (struct sockaddr *)&sin, len) == 0, "connect A");
		int sa = accept(ls, NULL, NULL);
		STEP(sa >= 0 && write(a, "A", 1) == 1, "accept/write A");
		close(a);
		usleep(20000);

		int b = socket(AF_INET, SOCK_STREAM, 0);
		STEP(connect(b, (struct sockaddr *)&sin, len) == 0, "connect B");
		STEP(write(b, "B", 1) == 1, "write B");
		usleep(20000);

		setsockopt(sa, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
		STEP(read(sa, &c, 1) == 1 && c == 'A', "read A");
		STEP(read(sa, &c, 1) == 0, "EOF A");
		close(sa);
		usleep(20000);

		int sb = accept(ls, NULL, NULL);
		setsockopt(sb, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
		setsockopt(b, SOL_SOCKET, SO_RCVTIMEO, &tv, sizeof(tv));
		STEP(sb >= 0 && read(sb, &c, 1) == 1 && c == 'B', "read B");
		STEP(write(sb, "b", 1) == 1 && read(b, &c, 1) == 1 && c == 'b', "reply B");
		close(sb);
		close(b);
	}
#undef STEP

	close(ls);

	if (step)
		printf("     step \"%s\" failed at iteration %d\n", step, i);

	CHECK("TCP connection reuse after peer close", !step);
}

static void test_time(void)
{
	struct timespec a, b;

	clock_gettime(CLOCK_MONOTONIC, &a);
	usleep(100000);
	clock_gettime(CLOCK_MONOTONIC, &b);

	long ms = (b.tv_sec - a.tv_sec) * 1000 + (b.tv_nsec - a.tv_nsec) / 1000000;
	CHECK("usleep 100ms", ms >= 100 && ms < 500);
	CHECK("realtime clock", time(NULL) > 1700000000);

	volatile double d = 0;
	for (int i = 1; i < 1000; i++)
		d += sqrt((double)i) * sin(i);
	CHECK("floating point", isfinite(d));

	// clock_gettime is served by the vDSO, values must interleave with
	// the system call ones
	int ok = 1;
	clockid_t clocks[] = {CLOCK_REALTIME, CLOCK_MONOTONIC};

	for (int c = 0; c < 2; c++) {
		for (int i = 0; i < 1000; i++) {
			struct timespec x, y, z;
			clock_gettime(clocks[c], &x);
			syscall(SYS_clock_gettime, clocks[c], &y);
			clock_gettime(clocks[c], &z);
			long long nx = x.tv_sec * 1000000000LL + x.tv_nsec;
			long long ny = y.tv_sec * 1000000000LL + y.tv_nsec;
			long long nz = z.tv_sec * 1000000000LL + z.tv_nsec;
			ok &= nx <= ny && ny <= nz && nz - nx < 1000000000LL;
		}
	}

	CHECK("vDSO mapped", getauxval(AT_SYSINFO_EHDR) != 0);
	CHECK("vDSO/syscall clocks agree", ok);
}

// Floating point state must survive system calls and switches between
// threads, each thread uses its own rounding mode and register values.
static void *fpu_thread(void *arg)
{
	long id = (long)arg;
	int modes[] = {FE_TONEAREST, FE_UPWARD, FE_DOWNWARD, FE_TOWARDZERO};
	int bad = 0;

	fesetround(modes[id]);

	for (int i = 0; i < 2000; i++) {
#ifdef __x86_64__
		double v = id * 1000 + i, sse;
		long double x87;
		long r;

		// XMM8 and the x87 stack hold values across a raw syscall
		// (sched_yield) which switches to the other threads
		__asm__ volatile (
			"movq %[v], %%xmm8\n\t"
			"fldl %[m]\n\t"
			"movl $24, %%eax\n\t"
			"syscall\n\t"
			"movq %%xmm8, %[sse]\n\t"
			"fstpt %[x87]\n\t"
			: [sse] "=x" (sse), [x87] "=m" (x87), "=a" (r)
			: [v] "r" (v), [m] "m" (v)
			: "rcx", "r11", "xmm8", "memory");
		bad |= sse != v || x87 != (long double)v;
#else
		sched_yield();
#endif
		bad |= fegetround() != modes[id];
	}

	return (void *)(long)bad;
}

static void test_fpu(void)
{
	pthread_t th[4];
	long bad = 0;

	for (long i = 0; i < 4; i++)
		pthread_create(&th[i], NULL, fpu_thread, (void *)i);

	for (int i = 0; i < 4; i++) {
		void *r;
		pthread_join(th[i], &r);
		bad |= (long)r;
	}

	CHECK("FPU state across syscalls and threads", bad == 0);
}

int main(void)
{
	setvbuf(stdout, NULL, _IOLBF, 0);

	test_fork_wait();
	test_pipe_exec();
	test_signals();
	test_threads();
	test_memory();
	test_files();
	test_poll();
	test_tcp_reuse();
	test_time();
	test_fpu();

	printf("\n%d passed, %d failed\n", passed, failed);

	return failed != 0;
}
