// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// tm runs a command with its output discarded and prints its wall clock
// duration as "PERF <name> <ms> ms", or "PERF <name> FAIL" on error.
#include <fcntl.h>
#include <stdio.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

int main(int argc, char **argv)
{
	struct timespec a, b;
	int st;

	if (argc < 3) {
		fprintf(stderr, "usage: tm name command [args]\n");
		return 2;
	}

	clock_gettime(CLOCK_MONOTONIC, &a);

	pid_t p = fork();

	if (p == 0) {
		int n = open("/dev/null", O_WRONLY);
		dup2(n, 1);
		dup2(n, 2);
		execvp(argv[2], argv + 2);
		_exit(127);
	}

	waitpid(p, &st, 0);
	clock_gettime(CLOCK_MONOTONIC, &b);

	if (!WIFEXITED(st) || WEXITSTATUS(st) != 0) {
		printf("PERF %s FAIL\n", argv[1]);
		return 1;
	}

	printf("PERF %s %.1f ms\n", argv[1],
	    (b.tv_sec - a.tv_sec) * 1e3 + (b.tv_nsec - a.tv_nsec) / 1e6);

	return 0;
}
