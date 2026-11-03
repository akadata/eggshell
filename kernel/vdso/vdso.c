// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// eggshell vDSO: time system calls served in user mode.
//
// The kernel maps this image in every process, preceded by a read-only data
// page (vvar) describing the clock, which it writes once at boot. Time is
// computed from the CPU counter with the same integer arithmetic as the kernel
// clock, so that values obtained here and through the system calls are
// identical.
//
// Built by build.sh (make), the resulting shared objects are embedded in the
// kernel.

#include <stdint.h>

#define ENOSYS 38

struct vvar {
	uint64_t cycle0;	// counter value at rt0
	uint64_t mult;		// nanoseconds per tick, 32.32 fixed point
	int64_t  rt0;		// CLOCK_REALTIME at cycle0
	int64_t  boot;		// CLOCK_REALTIME at boot (CLOCK_MONOTONIC origin)
};

struct timespec {
	int64_t tv_sec;
	long    tv_nsec;
};

struct timeval {
	int64_t tv_sec;
	long    tv_usec;
};

struct timezone {
	int tz_minuteswest;
	int tz_dsttime;
};

// defined by the linker script, one page before the image
extern const volatile struct vvar __vvar __attribute__((visibility("hidden")));

static inline uint64_t cycles(void)
{
#if defined(__x86_64__)
	uint32_t lo, hi;
	__asm__ volatile ("rdtsc" : "=a"(lo), "=d"(hi));
	return (uint64_t)hi << 32 | lo;
#else
#error unsupported architecture
#endif
}

static inline int64_t realtime(void)
{
	uint64_t d = cycles() - __vvar.cycle0;
	return __vvar.rt0 + (int64_t)(uint64_t)(((unsigned __int128)d * __vvar.mult) >> 32);
}

int __vdso_clock_gettime(int clk, struct timespec *ts)
{
	int64_t ns;

	switch (clk) {
	case 0: // CLOCK_REALTIME
	case 5: // CLOCK_REALTIME_COARSE
		ns = realtime();
		break;
	case 1: // CLOCK_MONOTONIC
	case 4: // CLOCK_MONOTONIC_RAW
	case 6: // CLOCK_MONOTONIC_COARSE
	case 7: // CLOCK_BOOTTIME
		ns = realtime() - __vvar.boot;
		break;
	default:
		// CPU time clocks are kept by the kernel
		return -ENOSYS;
	}

	ts->tv_sec = ns / 1000000000;
	ts->tv_nsec = ns % 1000000000;

	return 0;
}

int __vdso_gettimeofday(struct timeval *tv, struct timezone *tz)
{
	if (tv) {
		int64_t ns = realtime();
		tv->tv_sec = ns / 1000000000;
		tv->tv_usec = (ns % 1000000000) / 1000;
	}

	if (tz) {
		tz->tz_minuteswest = 0;
		tz->tz_dsttime = 0;
	}

	return 0;
}

int64_t __vdso_time(int64_t *t)
{
	int64_t s = realtime() / 1000000000;

	if (t)
		*t = s;

	return s;
}
