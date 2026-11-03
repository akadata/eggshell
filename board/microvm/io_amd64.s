// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

#include "textflag.h"

// func Out8(port uint16, val uint8)
TEXT ·Out8(SB),NOSPLIT,$0-3
	MOVW	port+0(FP), DX
	MOVB	val+2(FP), AX
	OUTB
	RET

// func Out16(port uint16, val uint16)
TEXT ·Out16(SB),NOSPLIT,$0-4
	MOVW	port+0(FP), DX
	MOVW	val+2(FP), AX
	OUTW
	RET

// func In8(port uint16) uint8
TEXT ·In8(SB),NOSPLIT,$0-9
	MOVW	port+0(FP), DX
	INB
	MOVB	AX, ret+8(FP)
	RET
