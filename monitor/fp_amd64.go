// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package monitor

import (
	"encoding/binary"
)

// FXSaveSize is the size of the FXSAVE format portion of FPState.
const FXSaveSize = 512

// FPState is the floating point state, in XSAVE standard format, sized for all
// components TamaGo can enable. It must be 64 byte aligned.
type FPState [4096]byte

// NewFPState returns an initialized floating point state.
func NewFPState() *FPState {
	fp := new(FPState)
	*fp = defaultFP
	return fp
}

// FSGSBASE reports whether user mode can read and write the FS and GS bases
// (RDFSBASE, WRFSBASE, RDGSBASE, WRGSBASE).
func FSGSBASE() bool {
	return hasFSGSBASE != 0
}

// NormalizeFPState writes the x87 and SSE state components that are in their
// initial configuration in the legacy region, as XSAVEOPT does not save them,
// so that it can be read (e.g. copied to a signal frame).
func NormalizeFPState(fp *FPState) {
	bv := fp[FXSaveSize]

	if bv&(1<<0) == 0 {
		clear(fp[0:24])   // FCW, FSW, FTW, FOP, FIP, FDP
		clear(fp[32:160]) // ST0-ST7
		binary.LittleEndian.PutUint16(fp[0:], 0x037f)
	}

	if bv&(1<<1) == 0 {
		clear(fp[160:416]) // XMM0-XMM15
	}

	fp[FXSaveSize] |= 1<<0 | 1<<1
}

// SanitizeFPState prepares a state whose legacy region has been modified (e.g.
// restored from a signal frame).
func SanitizeFPState(fp *FPState) {
	mxcsr := binary.LittleEndian.Uint32(fp[24:])
	binary.LittleEndian.PutUint32(fp[24:], mxcsr&0xffff)

	fp[FXSaveSize] |= 1<<0 | 1<<1
}
