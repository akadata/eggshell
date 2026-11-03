// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"encoding/binary"
	"math/bits"
	_ "unsafe"
)

// AT_SYSINFO_EHDR is the auxiliary vector entry holding the vDSO address.
const AT_SYSINFO_EHDR = 33

// vDSO placement: the vvar data page followed by the image, above the mmap
// region (findFree never allocates there).
const (
	vvarAddr = mmapTop
	vdsoAddr = vvarAddr + PageSize
)

// clock source, see SetClock
var clockSrc struct {
	counter func() uint64
	cycle0  uint64
	mult    uint64
	rt0     int64
}

// bootNano is the CLOCK_REALTIME value at boot, origin of CLOCK_MONOTONIC.
var bootNano int64

// vvarData represents the vDSO data page (struct vvar in vdso/vdso.c).
type vvarData struct {
	Cycle0 uint64
	Mult   uint64
	RT0    int64
	Boot   int64
}

// vDSO physical pages, mapped in every process
var vdsoPages struct {
	vvar  uint64
	image []uint64
}

//go:linkname runtimeNano runtime.nanotime
func runtimeNano() int64

// SetClock describes the board clock as a CPU counter, readable from user
// mode, and the time (nanoseconds since the Unix epoch) for a counter value c:
//
//	rt0 + int64((uint128(c - cycle0) * mult) >> 32)
//
// The kernel and the vDSO, which serves time from user mode, both compute time
// with this arithmetic so that their values are consistent.
func SetClock(counter func() uint64, cycle0 uint64, mult uint64, rt0 int64) {
	clockSrc.counter = counter
	clockSrc.cycle0 = cycle0
	clockSrc.mult = mult
	clockSrc.rt0 = rt0
}

// realtime returns CLOCK_REALTIME in nanoseconds.
func realtime() int64 {
	if clockSrc.counter == nil {
		return runtimeNano()
	}

	hi, lo := bits.Mul64(clockSrc.counter()-clockSrc.cycle0, clockSrc.mult)

	return clockSrc.rt0 + int64(hi<<32|lo>>32)
}

// nanotime returns the monotonic time since boot in nanoseconds.
func nanotime() int64 {
	return realtime() - bootNano
}

// vdsoInit loads the vDSO image and its data page in physical memory.
func vdsoInit() {
	if clockSrc.counter == nil || len(vdsoImage) == 0 {
		return
	}

	vvar, ok := allocPage()

	if !ok {
		return
	}

	data := &vvarData{
		Cycle0: clockSrc.cycle0,
		Mult:   clockSrc.mult,
		RT0:    clockSrc.rt0,
		Boot:   bootNano,
	}

	b := physPage(vvar)
	clear(b)
	binary.Encode(b, binary.LittleEndian, data)

	var image []uint64

	for off := 0; off < len(vdsoImage); off += PageSize {
		pa, ok := allocPage()

		if !ok {
			return
		}

		p := physPage(pa)
		clear(p)
		copy(p, vdsoImage[off:])
		image = append(image, pa)
	}

	// the initial references are never dropped
	vdsoPages.vvar = vvar
	vdsoPages.image = image
}

// mapVDSO maps the vDSO in the argument address space, it returns its
// address or 0 if unavailable.
func (mm *MM) mapVDSO() uint64 {
	if vdsoPages.vvar == 0 {
		return 0
	}

	mapShared := func(start uint64, pages []uint64, prot int, name string) bool {
		end := start + uint64(len(pages))*PageSize
		mm.removeRange(start, end)
		mm.insertVMA(&VMA{start: start, end: end, prot: prot, shared: true, name: name})

		for i, pa := range pages {
			getPage(pa)

			if !mm.mapPage(start+uint64(i)*PageSize, pa, prot) {
				putPage(pa)
				return false
			}
		}

		return true
	}

	if !mapShared(vvarAddr, []uint64{vdsoPages.vvar}, PROT_READ, "[vvar]") ||
		!mapShared(vdsoAddr, vdsoPages.image, PROT_READ|PROT_EXEC, "[vdso]") {
		return 0
	}

	return vdsoAddr
}
