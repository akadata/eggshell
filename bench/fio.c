// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// fio measures in-memory file throughput: 64 MiB written, rewritten in place
// and read back (best of three) for several I/O sizes.
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <unistd.h>

#define TOTAL (64 << 20)

static double now(void)
{
	struct timespec t;
	clock_gettime(CLOCK_MONOTONIC, &t);
	return t.tv_sec + t.tv_nsec / 1e9;
}

static int io(ssize_t (*f)(int, void *, size_t), int fd, char *buf, size_t n)
{
	for (size_t off = 0; off < TOTAL; off += n)
		if (f(fd, buf, n) != (ssize_t)n)
			return -1;
	return 0;
}

static ssize_t wr(int fd, void *b, size_t n) { return write(fd, b, n); }

int main(void)
{
	size_t sizes[] = {4096, 65536, 1 << 20};
	char *buf = malloc(1 << 20);

	memset(buf, 'x', 1 << 20);

	for (int i = 0; i < 3; i++) {
		size_t n = sizes[i];
		double t, w, rw, rd = 1e9;
		int fd = open("/tmp/fio.dat", O_CREAT | O_TRUNC | O_RDWR, 0644);

		t = now();
		if (io(wr, fd, buf, n)) goto fail;
		w = now() - t;

		lseek(fd, 0, SEEK_SET);
		t = now();
		if (io(wr, fd, buf, n)) goto fail;
		rw = now() - t;

		for (int k = 0; k < 3; k++) {
			lseek(fd, 0, SEEK_SET);
			t = now();
			if (io(read, fd, buf, n)) goto fail;
			if (now() - t < rd)
				rd = now() - t;
		}

		close(fd);
		unlink("/tmp/fio.dat");

		printf("BENCH file_write_%zuk %12.2f MB/s\n", n >> 10, TOTAL / w / 1e6);
		printf("BENCH file_rewrite_%zuk %12.2f MB/s\n", n >> 10, TOTAL / rw / 1e6);
		printf("BENCH file_read_%zuk %12.2f MB/s\n", n >> 10, TOTAL / rd / 1e6);
	}

	return 0;
fail:
	perror("fio");
	return 1;
}
