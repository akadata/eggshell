// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package microvm

import (
	"encoding/binary"
)

// defined in io_amd64.s
func Out16(port uint16, val uint16)
func In8(port uint16) uint8

// BootArgs returns the kernel command line (QEMU -append).
func BootArgs() string {
	Out16(FW_CFG_PORT_SEL, FW_CFG_CMDLINE_SIZE)

	var b [4]byte

	for i := range b {
		b[i] = In8(FW_CFG_PORT_DATA)
	}

	size := binary.LittleEndian.Uint32(b[:])

	if size == 0 || size > 4096 {
		return ""
	}

	Out16(FW_CFG_PORT_SEL, FW_CFG_CMDLINE_DATA)

	cmdline := make([]byte, 0, size)

	for range size {
		c := In8(FW_CFG_PORT_DATA)

		if c == 0 {
			break
		}

		cmdline = append(cmdline, c)
	}

	return string(cmdline)
}
