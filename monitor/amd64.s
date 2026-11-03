// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

#include "go_asm.h"
#include "textflag.h"

// func rdmsr(msr uint32) uint64
TEXT ·rdmsr(SB),NOSPLIT,$0-16
	MOVL	msr+0(FP), CX
	RDMSR
	SHLQ	$32, DX
	MOVL	AX, AX
	ORQ	DX, AX
	MOVQ	AX, ret+8(FP)
	RET

// func wrmsr(msr uint32, val uint64)
TEXT ·wrmsr(SB),NOSPLIT,$0-16
	MOVL	msr+0(FP), CX
	MOVQ	val+8(FP), AX
	MOVQ	AX, DX
	SHRQ	$32, DX
	WRMSR
	RET

// func lgdt(gdtr *[16]byte)
TEXT ·lgdt(SB),NOSPLIT,$0-8
	MOVQ	gdtr+0(FP), AX
	LGDT	(AX)
	RET

// func ltr(sel uint16)
TEXT ·ltr(SB),NOSPLIT,$0-2
	MOVW	sel+0(FP), AX
	LTR	AX
	RET

// func rdtsc() uint64
TEXT ·rdtsc(SB),NOSPLIT,$0-8
	RDTSC
	SHLQ	$32, DX
	MOVL	AX, AX
	ORQ	DX, AX
	MOVQ	AX, ret+0(FP)
	RET

// func readCR3() uint64
TEXT ·readCR3(SB),NOSPLIT,$0-8
	MOVQ	CR3, AX
	MOVQ	AX, ret+0(FP)
	RET

// func writeCR3(val uint64)
TEXT ·writeCR3(SB),NOSPLIT,$0-8
	MOVQ	val+0(FP), AX
	MOVQ	AX, CR3
	RET

// func readCR4() uint64
TEXT ·readCR4(SB),NOSPLIT,$0-8
	MOVQ	CR4, AX
	MOVQ	AX, ret+0(FP)
	RET

// func writeCR4(val uint64)
TEXT ·writeCR4(SB),NOSPLIT,$0-8
	MOVQ	val+0(FP), AX
	MOVQ	AX, CR4
	RET

// func write32(addr uint64, val uint32)
TEXT ·write32(SB),NOSPLIT,$0-12
	MOVQ	addr+0(FP), AX
	MOVL	val+8(FP), BX
	MOVL	BX, (AX)
	RET

// func idleHalt(count uint32)
TEXT ·idleHalt(SB),NOSPLIT,$0-4
	MOVL	count+0(FP), BX
	CLI
	MOVQ	$const_LAPIC_LVT_TIMER_INIT, AX
	MOVL	BX, (AX)

	STI
	HLT
	RET

// func xgetbv(index uint32) uint64
TEXT ·xgetbv(SB),NOSPLIT,$0-16
	MOVL	index+0(FP), CX
	XGETBV
	SHLQ	$32, DX
	MOVL	AX, AX
	ORQ	DX, AX
	MOVQ	AX, ret+8(FP)
	RET
