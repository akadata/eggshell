// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"encoding/binary"
)

// Signals
const (
	SIGHUP    = 1
	SIGINT    = 2
	SIGQUIT   = 3
	SIGILL    = 4
	SIGTRAP   = 5
	SIGABRT   = 6
	SIGBUS    = 7
	SIGFPE    = 8
	SIGKILL   = 9
	SIGUSR1   = 10
	SIGSEGV   = 11
	SIGUSR2   = 12
	SIGPIPE   = 13
	SIGALRM   = 14
	SIGTERM   = 15
	SIGSTKFLT = 16
	SIGCHLD   = 17
	SIGCONT   = 18
	SIGSTOP   = 19
	SIGTSTP   = 20
	SIGTTIN   = 21
	SIGTTOU   = 22
	SIGURG    = 23
	SIGXCPU   = 24
	SIGXFSZ   = 25
	SIGVTALRM = 26
	SIGPROF   = 27
	SIGWINCH  = 28
	SIGIO     = 29
	SIGPWR    = 30
	SIGSYS    = 31

	NSIG = 65
)

// sigaction flags
const (
	SA_NOCLDSTOP = 0x00000001
	SA_NOCLDWAIT = 0x00000002
	SA_SIGINFO   = 0x00000004
	SA_ONSTACK   = 0x08000000
	SA_RESTART   = 0x10000000
	SA_NODEFER   = 0x40000000
	SA_RESETHAND = 0x80000000

	SIG_DFL = 0
	SIG_IGN = 1

	SIG_BLOCK   = 0
	SIG_UNBLOCK = 1
	SIG_SETMASK = 2

	SS_ONSTACK = 1
	SS_DISABLE = 2
)

// siginfo codes
const (
	SI_USER       = 0
	SI_KERNEL     = 0x80
	SI_TKILL      = -6
	CLD_EXITED    = 1
	CLD_KILLED    = 2
	CLD_DUMPED    = 3
	CLD_STOPPED   = 5
	CLD_CONTINUED = 6

	SEGV_MAPERR = 1
	SEGV_ACCERR = 2
	BUS_ADRALN  = 1
	ILL_ILLOPC  = 1
	TRAP_BRKPT  = 1
)

// SigAction represents a signal disposition (struct k_sigaction).
type SigAction struct {
	Handler  uint64
	Flags    uint64
	Restorer uint64
	Mask     uint64
}

// Bytes converts the signal action structure to byte array format.
func (act *SigAction) Bytes() []byte {
	b, _ := binary.Append(nil, binary.LittleEndian, act)
	return b
}

// SigHand represents the signal handlers table shared by CLONE_SIGHAND.
type SigHand struct {
	act [NSIG]SigAction
}

// SigInfo represents signal information (siginfo_t).
type SigInfo struct {
	Signo  int
	Code   int
	Pid    int
	Status int
	Addr   uint64
}

// sigaltstack represents an alternate signal stack (stack_t).
type sigaltstack struct {
	Sp    uint64
	Flags int32
	_     int32
	Size  uint64
}

// Bytes converts the signal stack structure to byte array format.
func (s *sigaltstack) Bytes() []byte {
	b, _ := binary.Append(nil, binary.LittleEndian, s)
	return b
}

func sigbit(sig int) uint64 {
	return 1 << uint(sig-1)
}

const unblockable = 1<<(SIGKILL-1) | 1<<(SIGSTOP-1)

type defaultAction int

const (
	actTerm defaultAction = iota
	actIgn
	actCore
	actStop
	actCont
)

func sigDefault(sig int) defaultAction {
	switch sig {
	case SIGCHLD, SIGURG, SIGWINCH:
		return actIgn
	case SIGCONT:
		return actCont
	case SIGSTOP, SIGTSTP, SIGTTIN, SIGTTOU:
		return actStop
	case SIGQUIT, SIGILL, SIGTRAP, SIGABRT, SIGBUS, SIGFPE, SIGSEGV, SIGXCPU, SIGXFSZ, SIGSYS:
		return actCore
	}
	return actTerm
}

// ignored returns whether a signal would be discarded at generation.
func (p *Proc) ignored(sig int) bool {
	act := p.sigh.act[sig]

	if sig == SIGKILL || sig == SIGSTOP {
		return false
	}

	if act.Handler == SIG_IGN {
		return true
	}

	return act.Handler == SIG_DFL && sigDefault(sig) == actIgn
}

// signal sends a process directed signal.
func (p *Proc) signal(sig int, info SigInfo) {
	if sig <= 0 || sig >= NSIG || p.zombie {
		return
	}

	switch {
	case sig == SIGCONT || sig == SIGKILL:
		p.pending &^= sigbit(SIGSTOP) | sigbit(SIGTSTP) | sigbit(SIGTTIN) | sigbit(SIGTTOU)

		for _, t := range p.threads {
			t.pending &^= sigbit(SIGSTOP) | sigbit(SIGTSTP) | sigbit(SIGTTIN) | sigbit(SIGTTOU)
		}

		if p.stopped {
			p.stopped = false
			p.contReported = false

			if p.parent != nil && sig == SIGCONT {
				p.parent.signal(SIGCHLD, SigInfo{
					Signo:  SIGCHLD,
					Code:   CLD_CONTINUED,
					Pid:    p.pid,
					Status: SIGCONT,
				})
			}
		}
	case sigDefault(sig) == actStop:
		p.pending &^= sigbit(SIGCONT)
	}

	if p.ignored(sig) {
		wakeAll()
		return
	}

	if sig == SIGKILL {
		p.exitGroup(SIGKILL)
		return
	}

	p.pending |= sigbit(sig)
	p.info[sig] = info

	wakeAll()
}

// signal sends a thread directed signal.
func (t *Task) signal(sig int, info SigInfo) {
	p := t.proc

	if sig <= 0 || sig >= NSIG || p.zombie {
		return
	}

	if sig == SIGKILL || sig == SIGCONT || sigDefault(sig) == actStop {
		p.signal(sig, info)
		return
	}

	if p.ignored(sig) {
		return
	}

	t.pending |= sigbit(sig)
	t.info[sig] = info

	t.wakeForSignal()
}

// forceSignal delivers a synchronous fault signal which cannot be blocked
// or ignored.
func (t *Task) forceSignal(sig int, code int, addr uint64) {
	act := &t.proc.sigh.act[sig]

	if t.sigmask&sigbit(sig) != 0 || act.Handler == SIG_IGN {
		t.sigmask &^= sigbit(sig)
		act.Handler = SIG_DFL
	}

	t.pending |= sigbit(sig)
	t.info[sig] = SigInfo{Signo: sig, Code: code, Addr: addr}
}

// signalPending returns whether an unblocked signal is pending.
func (t *Task) signalPending() bool {
	return (t.pending|t.proc.pending)&^(t.sigmask&^unblockable) != 0 || t.proc.stopped
}

// nextSignal dequeues the lowest pending unblocked signal.
func (t *Task) nextSignal() (sig int, info SigInfo) {
	p := t.proc
	mask := t.sigmask &^ unblockable

	if set := t.pending &^ mask; set != 0 {
		sig = lowestSignal(set)
		t.pending &^= sigbit(sig)
		return sig, t.info[sig]
	}

	if set := p.pending &^ mask; set != 0 {
		sig = lowestSignal(set)
		p.pending &^= sigbit(sig)
		return sig, p.info[sig]
	}

	return 0, SigInfo{}
}

func lowestSignal(set uint64) int {
	for i := 1; i < NSIG; i++ {
		if set&sigbit(i) != 0 {
			return i
		}
	}
	return 0
}

// deliverSignals handles pending signals before returning to user mode.
func (t *Task) deliverSignals() {
	p := t.proc

	defer func() {
		// transparent system call restart
		if t.restart {
			t.restart = false
			t.restartSyscall()
		}

		if t.hasSavedMask {
			t.hasSavedMask = false
			t.sigmask = t.savedMask
		}
	}()

	for !t.dead {
		sig, info := t.nextSignal()

		if sig == 0 {
			return
		}

		act := p.sigh.act[sig]

		switch {
		case act.Handler == SIG_IGN:
			continue
		case act.Handler == SIG_DFL:
			switch sigDefault(sig) {
			case actIgn, actCont:
				continue
			case actStop:
				if p.stopped {
					continue
				}

				p.stopped = true
				p.stopSig = sig
				p.stopReported = false

				if p.parent != nil && p.parent.sigh.act[SIGCHLD].Flags&SA_NOCLDSTOP == 0 {
					p.parent.signal(SIGCHLD, SigInfo{
						Signo:  SIGCHLD,
						Code:   CLD_STOPPED,
						Pid:    p.pid,
						Status: sig,
					})
				}

				wakeAll()
				t.handleStop()
				continue
			case actCore:
				p.exitGroup(sig | 0x80)
				return
			default:
				p.exitGroup(sig)
				return
			}
		}

		// user handler
		if t.restart {
			t.restart = false
			t.sleepRestart = 0

			if act.Flags&SA_RESTART != 0 && !t.restartNoHand {
				t.restartSyscall()
			} else {
				t.setRet(negErrno(EINTR))
			}
		}

		mask := t.sigmask

		if t.hasSavedMask {
			mask = t.savedMask
			t.hasSavedMask = false
		}

		if err := t.setupFrame(sig, info, act, mask); err != 0 {
			p.exitGroup(SIGSEGV | 0x80)
			return
		}

		t.sigmask = mask | act.Mask

		if act.Flags&SA_NODEFER == 0 {
			t.sigmask |= sigbit(sig)
		}

		t.sigmask &^= unblockable

		if act.Flags&SA_RESETHAND != 0 {
			p.sigh.act[sig] = SigAction{}
		}

		// one handler frame per return to user mode
		return
	}
}

func (info *SigInfo) marshal() []byte {
	b := make([]byte, 128)
	le := binary.LittleEndian

	le.PutUint32(b[0:], uint32(info.Signo))
	le.PutUint32(b[8:], uint32(int32(info.Code)))

	switch {
	case info.Signo == SIGCHLD:
		le.PutUint32(b[16:], uint32(info.Pid))
		le.PutUint32(b[24:], uint32(info.Status))
	case info.Addr != 0 || info.Signo == SIGSEGV || info.Signo == SIGBUS || info.Signo == SIGILL:
		le.PutUint64(b[16:], info.Addr)
	default:
		le.PutUint32(b[16:], uint32(info.Pid))
	}

	return b
}

// kill implements kill(2) semantics.
func (t *Task) kill(pid int, sig int) Errno {
	if sig < 0 || sig >= NSIG {
		return EINVAL
	}

	info := SigInfo{Signo: sig, Code: SI_USER, Pid: t.proc.pid}

	var targets []*Proc

	switch {
	case pid > 0:
		p, ok := procs[pid]

		if !ok || p.zombie {
			if tk, ok := tasks[pid]; ok {
				p = tk.proc
			} else {
				return ESRCH
			}
		}

		targets = append(targets, p)
	case pid == 0:
		targets = pgrpMembers(t.proc.pgid)
	case pid == -1:
		for _, p := range procs {
			if p.pid != 1 && p != t.proc {
				targets = append(targets, p)
			}
		}
	default:
		targets = pgrpMembers(-pid)
	}

	if len(targets) == 0 {
		return ESRCH
	}

	if sig == 0 {
		return 0
	}

	for _, p := range targets {
		p.signal(sig, info)
	}

	return 0
}

func pgrpMembers(pgid int) (members []*Proc) {
	for _, p := range procs {
		if p.pgid == pgid && !p.zombie {
			members = append(members, p)
		}
	}
	return
}

// signalPgrp sends a signal to all members of a process group.
func signalPgrp(pgid int, sig int) {
	for _, p := range pgrpMembers(pgid) {
		p.signal(sig, SigInfo{Signo: sig, Code: SI_KERNEL})
	}
}
