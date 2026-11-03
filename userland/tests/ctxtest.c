/*
 * User mode context switch test for TamaGo POSIX unikernels.
 *
 * Checks the per thread state saved and restored by the kernel on traps:
 * FS base (TLS), GS base set from user mode (FSGSBASE) and the floating point
 * state copied to signal frames, printing one line per check and a final
 * summary.
 */
#define _GNU_SOURCE
#include <pthread.h>
#include <sched.h>
#include <signal.h>
#include <stdio.h>
#include <string.h>
#include <sys/auxv.h>
#include <ucontext.h>
#include <unistd.h>

#define HWCAP2_FSGSBASE (1 << 1)

static int passed, failed;

#define CHECK(name, cond) do { \
	if (cond) { passed++; printf("ok   %s\n", name); } \
	else { failed++; printf("FAIL %s\n", name); } \
} while (0)

static __thread long tls;
static volatile int gs_bad, tls_bad;

/*
 * Each thread sets its own GS base and TLS value, which must survive system
 * calls switching to the other threads.
 */
static void *base_thread(void *arg)
{
	unsigned long v = (unsigned long)arg, r;

	tls = (long)(v & 0xff);
	__asm__ volatile ("wrgsbase %0" :: "r"(v));

	for (int i = 0; i < 20000; i++) {
		sched_yield();
		__asm__ volatile ("rdgsbase %0" : "=r"(r));

		if (r != v)
			gs_bad = 1;

		if (tls != (long)(v & 0xff))
			tls_bad = 1;
	}

	return NULL;
}

static volatile int frame_stale;

static void handler(int sig, siginfo_t *si, void *ucp)
{
	ucontext_t *uc = ucp;
	unsigned char *xmm = (unsigned char *)uc->uc_mcontext.fpregs->_xmm;

	for (int i = 0; i < 16 * 16; i++)
		if (xmm[i])
			frame_stale = 1;

	// clobbered registers must be restored on return
	__asm__ volatile ("pcmpeqb %%xmm3, %%xmm3" ::: "xmm3");
}

static int has_avx(void)
{
	unsigned int eax = 1, ebx, ecx = 0, edx;

	__asm__ volatile ("cpuid" : "+a"(eax), "=b"(ebx), "+c"(ecx), "=d"(edx));

	// AVX and OSXSAVE
	return (ecx & (1 << 28 | 1 << 27)) == (1 << 28 | 1 << 27);
}

int main(void)
{
	setvbuf(stdout, NULL, _IONBF, 0);

	tls = 42;
	CHECK("TLS (FS base)", tls == 42);

	if (getauxval(AT_HWCAP2) & HWCAP2_FSGSBASE) {
		pthread_t t[2];

		pthread_create(&t[0], NULL, base_thread, (void *)0x111100000001UL);
		pthread_create(&t[1], NULL, base_thread, (void *)0x222200000002UL);
		pthread_join(t[0], NULL);
		pthread_join(t[1], NULL);

		CHECK("GS base per thread (WRGSBASE)", !gs_bad);
		CHECK("TLS per thread", !tls_bad && tls == 42);
	} else {
		printf("skip GS base per thread (no HWCAP2_FSGSBASE)\n");
	}

	struct sigaction sa = {.sa_sigaction = handler, .sa_flags = SA_SIGINFO};
	sigaction(SIGUSR1, &sa, NULL);

	// Save dirty SSE registers on a system call, then return them to their
	// initial configuration, which the kernel might not save again, before
	// a signal: the frame must not expose the earlier values.
	__asm__ volatile (
		"pcmpeqb %%xmm0, %%xmm0\n\t"
		"pcmpeqb %%xmm7, %%xmm7\n\t"
		"pcmpeqb %%xmm15, %%xmm15"
		::: "xmm0", "xmm7", "xmm15");
	sched_yield();

	unsigned long r;
	long pid = getpid();

#define CLEAR_SSE \
	"pxor %%xmm0, %%xmm0\n\t pxor %%xmm1, %%xmm1\n\t pxor %%xmm2, %%xmm2\n\t" \
	"pxor %%xmm3, %%xmm3\n\t pxor %%xmm4, %%xmm4\n\t pxor %%xmm5, %%xmm5\n\t" \
	"pxor %%xmm6, %%xmm6\n\t pxor %%xmm7, %%xmm7\n\t pxor %%xmm8, %%xmm8\n\t" \
	"pxor %%xmm9, %%xmm9\n\t pxor %%xmm10, %%xmm10\n\t pxor %%xmm11, %%xmm11\n\t" \
	"pxor %%xmm12, %%xmm12\n\t pxor %%xmm13, %%xmm13\n\t pxor %%xmm14, %%xmm14\n\t" \
	"pxor %%xmm15, %%xmm15\n\t"

#define KILL_SELF \
	"movl $62, %%eax\n\t"	/* kill(pid, SIGUSR1) */ \
	"movq %1, %%rdi\n\t" \
	"movq $10, %%rsi\n\t" \
	"syscall\n\t" \
	"movq %%xmm3, %0\n\t"

#define CLOBBERS "rax", "rdi", "rsi", "rcx", "r11", "memory", \
	"xmm0", "xmm1", "xmm2", "xmm3", "xmm4", "xmm5", "xmm6", "xmm7", \
	"xmm8", "xmm9", "xmm10", "xmm11", "xmm12", "xmm13", "xmm14", "xmm15"

	if (has_avx())
		__asm__ volatile ("vzeroall\n\t" KILL_SELF : "=r"(r) : "r"(pid) : CLOBBERS);
	else
		__asm__ volatile (CLEAR_SSE KILL_SELF : "=r"(r) : "r"(pid) : CLOBBERS);

	CHECK("signal frame XMM state", !frame_stale);
	CHECK("XMM restored after handler", r == 0);

	printf("\n%d passed, %d failed\n", passed, failed);

	return failed != 0;
}
