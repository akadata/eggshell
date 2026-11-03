// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import "sync"

// Buffers for data transfer system calls (read, write and their variants) are
// reused rather than allocated on each call: file contents are outside Go
// runtime memory (see filedata.go), which keeps the heap small, so per call
// garbage would cause frequent garbage collection.
//
// File operations copy the data they are passed before returning, and write
// all the data they return, so buffers are never retained or partially stale.
//
// Buffers come in size classes, so that a task blocked with a small buffer
// does not hold a large one.
var ioBufSizes = [...]uint64{4 << 10, 64 << 10, ioChunk}

var ioBufPools [len(ioBufSizes)]sync.Pool

// getIOBuf returns a buffer of n bytes, to be released with putIOBuf, buffers
// larger than ioChunk are not pooled.
func getIOBuf(n uint64) ([]byte, *[]byte) {
	for i, size := range ioBufSizes {
		if n > size {
			continue
		}

		p, _ := ioBufPools[i].Get().(*[]byte)

		if p == nil {
			b := make([]byte, size)
			p = &b
		}

		return (*p)[:n], p
	}

	return make([]byte, n), nil
}

func putIOBuf(p *[]byte) {
	if p == nil {
		return
	}

	for i, size := range ioBufSizes {
		if uint64(cap(*p)) == size {
			ioBufPools[i].Put(p)
			return
		}
	}
}
