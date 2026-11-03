/*
 * System call round trip decomposition. The probe numbers are answered by the
 * Go monitor (0x7ff1 trap entry assembly, 0x7ff2 Go monitor), on Linux they
 * take the ENOSYS path.
 */

#include <stdio.h>
#include <sys/syscall.h>
#include <time.h>
#include <unistd.h>

#ifndef __x86_64__
#define __x86_64__ 0
#endif

static double now(void)
{
	struct timespec ts;
	clock_gettime(CLOCK_MONOTONIC, &ts);
	return ts.tv_sec + ts.tv_nsec / 1e9;
}

static void probe(const char *name, long nr)
{
	int n = 50000;
	double best = 1e9;

	for (int r = 0; r < 5; r++) {
		double t = now();
		for (int i = 0; i < n; i++)
			syscall(nr);
		t = (now() - t) / n * 1e9;
		if (t < best)
			best = t;
	}

	printf("PROBE %-12s %8.0f ns\n", name, best);
}

int main(void)
{
	/* user mode loop baseline (call overhead without trapping) */
	volatile long sink = 0;
	double t = now();
	for (int i = 0; i < 50000; i++)
		sink += i;
	printf("PROBE %-12s %8.0f ns\n", "user_loop", (now() - t) / 50000 * 1e9);

	probe("trap_asm", 0x7ff1);

	if (sizeof(void *) == 8 && __x86_64__) {
		probe("+fxsave2x", 0x7ff3);
		probe("+fsbase2x", 0x7ff4);
		probe("+lidt2x", 0x7ff5);
		probe("+iretq", 0x7ff6);
		probe("+fs_sel2x", 0x7ff7);
		probe("+cr3", 0x7ff8);
	}

	probe("monitor_go", 0x7ff2);
	probe("getppid", SYS_getppid);
	probe("getpid", SYS_getpid);

	return 0;
}
