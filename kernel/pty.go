// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"errors"
	"io"
	"strconv"
)

// pseudo terminal output buffered for the master side before writers block
const ptyBufSize = 65536

// PTY represents the master side of a pseudo terminal, used by kernel
// services (e.g. the SSH server) to drive a terminal session. Its slave side
// is the controlling terminal and standard streams of the process started
// with it (see [Cmd]), also available as /dev/pts/<n>.
type PTY struct {
	tty  *TTY
	name string
}

// ptsDev implements the pseudo terminal slave device.
type ptsDev struct {
	tty *TTY
}

func (d ptsDev) Open(t *Task, ino *Inode, flags int, path string) (*File, Errno) {
	p := t.proc
	tty := d.tty

	if tty.hungup {
		return nil, EIO
	}

	// acquire controlling terminal
	if flags&O_NOCTTY == 0 && p.sid == p.pid && tty.sid == 0 && ctty(p) == nil {
		tty.sid = p.sid
		tty.pgrp = p.pgid
	}

	return newFile(&ttyFile{tty: tty}, ino, flags, path), 0
}

// OpenPTY allocates a pseudo terminal.
func OpenPTY() (*PTY, error) {
	klock.Lock()
	defer klock.Unlock()

	tty := &TTY{
		rows:    24,
		cols:    80,
		termios: defaultTermios(),
		master:  true,
	}

	dir := mkdirAll("/dev/pts", 0755)
	n := 0

	for dir.children[strconv.Itoa(n)] != nil {
		n++
	}

	ino := mkdev(dir, strconv.Itoa(n), 136, uint64(n), ptsDev{tty: tty})
	ino.mode = S_IFCHR | 0620

	ttys = append(ttys, tty)

	return &PTY{tty: tty, name: strconv.Itoa(n)}, nil
}

// Name returns the slave device path.
func (pty *PTY) Name() string {
	return "/dev/pts/" + pty.name
}

// open returns a file for the slave side.
func (pty *PTY) open() *File {
	ino := mkdirAll("/dev/pts", 0755).children[pty.name]
	return newFile(&ttyFile{tty: pty.tty}, ino, O_RDWR, pty.Name())
}

// Read reads terminal output, it returns [io.EOF] once the terminal is
// closed and its output consumed.
func (pty *PTY) Read(buf []byte) (n int, err error) {
	klock.Lock()
	defer klock.Unlock()

	tty := pty.tty

	for len(tty.out) == 0 {
		if tty.hungup {
			return 0, io.EOF
		}

		kcond.Wait()
	}

	n = copy(buf, tty.out)
	tty.out = tty.out[n:]

	if len(tty.out) == 0 {
		tty.out = nil
	}

	// writers waiting for buffer space
	wakeAll()

	return
}

// Write writes terminal input, processed by the line discipline.
func (pty *PTY) Write(buf []byte) (n int, err error) {
	klock.Lock()
	defer klock.Unlock()

	if pty.tty.hungup {
		return 0, errors.New("terminal closed")
	}

	for _, c := range buf {
		pty.tty.receive(c)
	}

	wakeAll()

	return len(buf), nil
}

// SetSize sets the terminal window size, notifying its foreground process
// group.
func (pty *PTY) SetSize(cols int, rows int) {
	klock.Lock()
	defer klock.Unlock()

	tty := pty.tty
	tty.cols = uint16(cols)
	tty.rows = uint16(rows)

	if tty.pgrp > 0 {
		signalPgrp(tty.pgrp, SIGWINCH)
	}
}

// Close hangs up the terminal: its session is sent SIGHUP, reads from its
// slave side return end-of-file and writes fail.
func (pty *PTY) Close() error {
	klock.Lock()
	defer klock.Unlock()

	tty := pty.tty

	if tty.hungup {
		return nil
	}

	tty.hungup = true

	if p := procs[tty.sid]; p != nil && tty.sid > 0 {
		p.signal(SIGHUP, SigInfo{Signo: SIGHUP})
		p.signal(SIGCONT, SigInfo{Signo: SIGCONT})
	}

	if tty.pgrp > 0 {
		signalPgrp(tty.pgrp, SIGHUP)
	}

	for i, t := range ttys {
		if t == tty {
			ttys = append(ttys[:i], ttys[i+1:]...)
			break
		}
	}

	mkdirAll("/dev/pts", 0755).unlink(pty.name)

	wakeAll()

	return nil
}
