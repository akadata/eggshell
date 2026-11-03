// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package microvm

import (
	_ "unsafe"
)

// Memory layout (the machine must be started with `-m 2G`)
const (
	// MemorySize is the expected amount of physical RAM.
	MemorySize = 0x80000000

	// Go runtime memory, starting at the TamaGo amd64 ramStart.
	RamStart = 0x10000000
	RamSize  = 0x20000000

	// DMA region (VirtIO queues and buffers)
	DMAStart = RamStart + RamSize
	DMASize  = 0x01000000

	// UserStart and UserSize describe the physical page pool for POSIX
	// processes.
	UserStart = DMAStart + DMASize
	UserSize  = MemorySize - UserStart
)

//go:linkname ramSize runtime/goos.RamSize
var ramSize uint64 = RamSize
