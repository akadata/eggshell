/* Round trip latency over pipes, unix sockets and loopback TCP. */

#include <arpa/inet.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <stdio.h>
#include <stdlib.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#define N 2000

static double now(void)
{
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec + ts.tv_nsec / 1e9;
}

static void bench(const char *name, int a_rd, int a_wr, int b_rd, int b_wr)
{
	char c = 'x';
	pid_t pid = fork();

	if (pid == 0) {
		for (int i = 0; i < N; i++) {
			if (read(b_rd, &c, 1) != 1) _exit(1);
			write(b_wr, &c, 1);
		}
		_exit(0);
	}

	double t = now();

	for (int i = 0; i < N; i++) {
		write(a_wr, &c, 1);
		if (read(a_rd, &c, 1) != 1) { perror("read"); break; }
	}

	t = now() - t;
	waitpid(pid, NULL, 0);
	printf("%-12s %8.1f us/roundtrip\n", name, t / N * 1e6);
}

int main(void)
{
	int p1[2], p2[2], sv[2];

	pipe(p1);
	pipe(p2);
	bench("pipe", p2[0], p1[1], p1[0], p2[1]);

	socketpair(AF_UNIX, SOCK_STREAM, 0, sv);
	bench("unix", sv[0], sv[0], sv[1], sv[1]);

	int l = socket(AF_INET, SOCK_STREAM, 0), one = 1;
	struct sockaddr_in sa = {.sin_family = AF_INET, .sin_port = htons(7777)};
	sa.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
	setsockopt(l, SOL_SOCKET, SO_REUSEADDR, &one, sizeof(one));

	if (bind(l, (void *)&sa, sizeof(sa)) || listen(l, 4)) {
		perror("bind/listen");
		return 1;
	}

	int c = socket(AF_INET, SOCK_STREAM, 0);

	if (connect(c, (void *)&sa, sizeof(sa))) {
		perror("connect");
		return 1;
	}

	int s = accept(l, NULL, NULL);
	setsockopt(c, IPPROTO_TCP, TCP_NODELAY, &one, sizeof(one));
	setsockopt(s, IPPROTO_TCP, TCP_NODELAY, &one, sizeof(one));
	bench("tcp loopback", c, c, s, s);

	return 0;
}
