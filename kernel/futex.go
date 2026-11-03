// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

const (
	FUTEX_WAIT             = 0
	FUTEX_WAKE             = 1
	FUTEX_REQUEUE          = 3
	FUTEX_CMP_REQUEUE      = 4
	FUTEX_WAKE_OP          = 5
	FUTEX_WAIT_BITSET      = 9
	FUTEX_WAKE_BITSET      = 10
	FUTEX_PRIVATE_FLAG     = 128
	FUTEX_CLOCK_REALTIME   = 256
	FUTEX_BITSET_MATCH_ANY = 0xffffffff
)

type futexWaiter struct {
	key    uint64
	bitset uint32
	woken  bool

	// the waiting task
	q waitQueue
}

var futexQueue []*futexWaiter

func futexKey(mm *MM, uaddr uint64) (uint64, Errno) {
	if uaddr&3 != 0 {
		return 0, EINVAL
	}

	return mm.translate(uaddr, false)
}

func futexWake(mm *MM, uaddr uint64, n int, bitset uint32) (int, Errno) {
	key, err := futexKey(mm, uaddr)

	if err != 0 {
		return 0, err
	}

	woken := 0

	for _, w := range futexQueue {
		if woken >= n {
			break
		}

		if w.key == key && !w.woken && w.bitset&bitset != 0 {
			w.woken = true
			w.q.wake()
			woken++
		}
	}

	if woken > 0 {
		futexPrune()
	}

	return woken, 0
}

func futexPrune() {
	q := futexQueue[:0]

	for _, w := range futexQueue {
		if !w.woken {
			q = append(q, w)
		}
	}

	futexQueue = q
}

func (t *Task) futexWait(uaddr uint64, val uint32, deadline int64, bitset uint32) Errno {
	mm := t.proc.mm

	if bitset == 0 {
		return EINVAL
	}

	key, err := futexKey(mm, uaddr)

	if err != 0 {
		return err
	}

	cur, err := mm.readU32(uaddr)

	if err != 0 {
		return err
	}

	if cur != val {
		return EAGAIN
	}

	w := &futexWaiter{key: key, bitset: bitset}
	futexQueue = append(futexQueue, w)

	err = t.blockOn(&w.q, func() bool { return w.woken }, deadline, true)

	if !w.woken {
		w.woken = true
		futexPrune()
	}

	if err == ERESTARTSYS {
		return EINTR
	}

	return err
}

func (t *Task) futexRequeue(uaddr uint64, n int, uaddr2 uint64, n2 int, cmp bool, val uint32) (int, Errno) {
	mm := t.proc.mm

	if cmp {
		cur, err := mm.readU32(uaddr)

		if err != 0 {
			return 0, err
		}

		if cur != val {
			return 0, EAGAIN
		}
	}

	key, err := futexKey(mm, uaddr)

	if err != 0 {
		return 0, err
	}

	key2, err := futexKey(mm, uaddr2)

	if err != 0 {
		return 0, err
	}

	woken := 0
	moved := 0

	for _, w := range futexQueue {
		if w.key != key || w.woken {
			continue
		}

		if woken < n {
			w.woken = true
			w.q.wake()
			woken++
		} else if moved < n2 {
			w.key = key2
			moved++
		}
	}

	futexPrune()

	return woken + moved, 0
}

func (t *Task) futexWakeOp(uaddr uint64, n int, uaddr2 uint64, n2 int, op uint32) (int, Errno) {
	mm := t.proc.mm

	old, err := mm.readU32(uaddr2)

	if err != 0 {
		return 0, err
	}

	oparg := (op >> 12) & 0xfff
	cmparg := op & 0xfff

	if (op>>28)&8 != 0 {
		oparg = 1 << oparg
	}

	var nv uint32

	switch (op >> 28) & 7 {
	case 0:
		nv = oparg
	case 1:
		nv = old + oparg
	case 2:
		nv = old | oparg
	case 3:
		nv = old &^ oparg
	case 4:
		nv = old ^ oparg
	}

	if err = mm.writeU32(uaddr2, nv); err != 0 {
		return 0, err
	}

	r, _ := futexWake(mm, uaddr, n, FUTEX_BITSET_MATCH_ANY)

	var cond bool
	o, c := int32(old), int32(cmparg)

	switch (op >> 24) & 0xf {
	case 0:
		cond = o == c
	case 1:
		cond = o != c
	case 2:
		cond = o < c
	case 3:
		cond = o <= c
	case 4:
		cond = o > c
	case 5:
		cond = o >= c
	}

	if cond {
		r2, _ := futexWake(mm, uaddr2, n2, FUTEX_BITSET_MATCH_ANY)
		r += r2
	}

	return r, 0
}
