// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package monitor

import (
	"github.com/usbarmory/tamago/amd64"
	"github.com/usbarmory/tamago/amd64/lapic"
)

const LAPIC_LVT_TIMER_INIT = amd64.LAPIC_BASE + lapic.LAPIC_LVT_TIMER_INIT

// defined in amd64.s
func rdmsr(msr uint32) uint64
func wrmsr(msr uint32, val uint64)
func lgdt(gdtr *[16]byte)
func ltr(sel uint16)
func rdtsc() uint64
func readCR3() uint64
func writeCR3(val uint64)
func readCR4() uint64
func writeCR4(val uint64)
func write32(addr uint64, val uint32)
func idleHalt(count uint32)
func xgetbv(index uint32) uint64
