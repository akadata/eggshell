// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"fmt"
	"sync/atomic"
)

type syscallFn func(t *Task, a *[6]uint64) (uint64, Errno)

// system call handlers indexed by number
var syscallTable [512]syscallFn

func init() {
	registerProcSyscalls()
	registerFSSyscalls()
	registerMMSyscalls()
	registerPollSyscalls()
	registerNetSyscalls()

	// architecture specific and legacy system calls, which might alias
	// the generic ones
	registerArchSyscalls()
}

func reg(num uint64, fn syscallFn) {
	syscallTable[num] = fn
}

// syscall services a system call trapped from user mode, following the Linux
// ABI of the running architecture (see arch_*.go).
func (t *Task) syscall() {
	syscallCount.Add(1)
	num := t.sysno()
	a := t.sysargs()

	current = t
	t.saveRestartState()
	t.noReturn = false
	t.restart = false

	var ret uint64
	var err Errno

	var fn syscallFn

	if num < uint64(len(syscallTable)) {
		fn = syscallTable[num]
	}

	if fn != nil {
		ret, err = fn(t, &a)
	} else {
		if cfg.Strace || debugUnknown {
			printk("[%d] unimplemented syscall %s (%d)\n", t.tid, syscallName(num), num)
		}
		err = ENOSYS
	}

	if cfg.Strace {
		t.trace(num, &a, ret, err)
	}

	if t.noReturn {
		return
	}

	switch err {
	case 0:
		t.setRet(ret)
	case ERESTARTSYS, ERESTARTNOHAND:
		t.restart = true
		t.restartNoHand = err == ERESTARTNOHAND
		t.setRet(negErrno(EINTR))
	default:
		t.setRet(negErrno(err))
	}
}

var debugUnknown = false

// syscallCount counts serviced system calls.
var syscallCount atomic.Uint64

func syscallName(num uint64) string {
	if name, ok := syscallNames[num]; ok {
		return name
	}

	return fmt.Sprintf("syscall_%d", num)
}

func (t *Task) trace(num uint64, a *[6]uint64, ret uint64, err Errno) {
	res := fmt.Sprintf("%d", int64(ret))

	if err != 0 {
		res = fmt.Sprintf("-1 %s", err)
	}

	printk("[%d %s] %s(%#x, %#x, %#x, %#x) = %s\n",
		t.tid,
		t.proc.comm,
		syscallName(num),
		a[0], a[1], a[2], a[3],
		res,
	)
}

func (t *Task) str(addr uint64) (string, Errno) {
	if addr == 0 {
		return "", EFAULT
	}
	return t.proc.mm.readString(addr, 4096)
}

func (t *Task) strArray(addr uint64) (out []string, err Errno) {
	if addr == 0 {
		return nil, 0
	}

	for i := range 65536 {
		p, err := t.proc.mm.readU64(addr + uint64(i)*8)

		if err != 0 {
			return nil, err
		}

		if p == 0 {
			return out, 0
		}

		s, err := t.proc.mm.readString(p, 131072)

		if err != 0 {
			return nil, err
		}

		out = append(out, s)
	}

	return nil, E2BIG
}

func (t *Task) file(fd uint64) (*File, Errno) {
	f := t.proc.files.get(int(int32(fd)))

	if f == nil {
		return nil, EBADF
	}

	return f, 0
}

func ok(v int) (uint64, Errno) {
	return uint64(int64(v)), 0
}

func fail(err Errno) (uint64, Errno) {
	return 0, err
}

func negErrno(err Errno) uint64 {
	return uint64(-int64(err))
}
