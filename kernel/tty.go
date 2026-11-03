// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"encoding/binary"
	"time"
)

// termios flags (asm-generic/termbits.h)
const (
	IGNBRK  = 0x001
	BRKINT  = 0x002
	INLCR   = 0x040
	IGNCR   = 0x080
	ICRNL   = 0x100
	IXON    = 0x400
	IUTF8   = 0x4000
	OPOST   = 0x01
	ONLCR   = 0x04
	ISIG    = 0x001
	ICANON  = 0x002
	ECHO    = 0x008
	ECHOE   = 0x010
	ECHOK   = 0x020
	ECHONL  = 0x040
	NOFLSH  = 0x080
	TOSTOP  = 0x100
	ECHOCTL = 0x200
	ECHOKE  = 0x800
	IEXTEN  = 0x8000

	VINTR   = 0
	VQUIT   = 1
	VERASE  = 2
	VKILL   = 3
	VEOF    = 4
	VTIME   = 5
	VMIN    = 6
	VSUSP   = 10
	VEOL    = 11
	VWERASE = 14
	VLNEXT  = 15
	VEOL2   = 16

	NCCS = 19
)

// ioctl requests
const (
	TCGETS     = 0x5401
	TCSETS     = 0x5402
	TCSETSW    = 0x5403
	TCSETSF    = 0x5404
	TCSBRK     = 0x5409
	TCXONC     = 0x540a
	TCFLSH     = 0x540b
	TIOCSCTTY  = 0x540e
	TIOCGPGRP  = 0x540f
	TIOCSPGRP  = 0x5410
	TIOCOUTQ   = 0x5411
	TIOCGWINSZ = 0x5413
	TIOCSWINSZ = 0x5414
	FIONREAD   = 0x541b
	TIOCNOTTY  = 0x5422
	TIOCGSID   = 0x5429
	FIONBIO    = 0x5421
	FIOCLEX    = 0x5451
	FIONCLEX   = 0x5450
	FIOASYNC   = 0x5452
)

// termios represents the struct termios (TCGETS/TCSETS).
type termios struct {
	Iflag uint32
	Oflag uint32
	Cflag uint32
	Lflag uint32
	Line  uint8
	Cc    [NCCS]uint8
}

// Bytes converts the termios structure to byte array format.
func (t *termios) Bytes() []byte {
	b, _ := binary.Append(nil, binary.LittleEndian, t)
	return b
}

func (t *termios) unmarshal(b []byte) (err error) {
	_, err = binary.Decode(b, binary.LittleEndian, t)
	return
}

// winsize represents the struct winsize (TIOCGWINSZ/TIOCSWINSZ).
type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

// Bytes converts the window size structure to byte array format.
func (w *winsize) Bytes() []byte {
	b, _ := binary.Append(nil, binary.LittleEndian, w)
	return b
}

// TTY represents a terminal with a line discipline, either the serial console
// or a pseudo terminal (see [PTY]).
type TTY struct {
	termios termios

	// cooked input ready for reading
	input []byte
	// canonical mode line being edited
	line []byte
	// pending end-of-file conditions (canonical mode)
	eof int

	rows, cols uint16

	sid  int
	pgrp int

	// pseudo terminal output, pending for the master side
	master bool
	out    []byte

	hungup bool
}

var console = &TTY{
	rows:    24,
	cols:    80,
	termios: defaultTermios(),
}

// ttys holds the terminals which can be a controlling terminal.
var ttys = []*TTY{console}

// ctty returns the controlling terminal of the argument process session.
func ctty(p *Proc) *TTY {
	for _, tty := range ttys {
		if tty.sid != 0 && tty.sid == p.sid {
			return tty
		}
	}

	return nil
}

func defaultTermios() termios {
	return termios{
		Iflag: ICRNL | IXON | IUTF8,
		Oflag: OPOST | ONLCR,
		Cflag: 0xbf, // B38400 | CS8 | CREAD
		Lflag: ISIG | ICANON | ECHO | ECHOE | ECHOK | ECHOCTL | ECHOKE | IEXTEN,
		Cc: [NCCS]uint8{
			VINTR: 3, VQUIT: 0x1c, VERASE: 0x7f, VKILL: 0x15, VEOF: 4,
			VTIME: 0, VMIN: 1, 8: 0x11, 9: 0x13, VSUSP: 0x1a, 12: 0x12,
			13: 0x0f, VWERASE: 0x17, VLNEXT: 0x16,
		},
	}
}

// poll reads characters from the console.
func (tty *TTY) poll() {
	for {
		var buf []byte

		for {
			c, ok := cfg.Console.Rx(false)

			if !ok {
				break
			}

			buf = append(buf, c)
		}

		if len(buf) > 0 {
			klock.Lock()

			for _, c := range buf {
				tty.receive(c)
			}

			wakeAll()
			klock.Unlock()
		}

		time.Sleep(2 * time.Millisecond)
	}
}

func (tty *TTY) hangup() {
	tty.sid = 0
	tty.pgrp = 0
}

func (tty *TTY) canonical() bool {
	return tty.termios.Lflag&ICANON != 0
}

// output writes processed characters to the console, or queues them for the
// pseudo terminal master.
func (tty *TTY) output(buf []byte) {
	opost := tty.termios.Oflag&OPOST != 0 && tty.termios.Oflag&ONLCR != 0

	if tty.master {
		for _, c := range buf {
			if c == '\n' && opost {
				tty.out = append(tty.out, '\r')
			}
			tty.out = append(tty.out, c)
		}

		wakeAll()
		return
	}

	for _, c := range buf {
		if c == '\n' && opost {
			cfg.Console.Tx('\r')
		}
		cfg.Console.Tx(c)
	}
}

func (tty *TTY) echo(c byte) {
	lflag := tty.termios.Lflag

	if lflag&ECHO == 0 {
		if c == '\n' && lflag&ECHONL != 0 && lflag&ICANON != 0 {
			tty.output([]byte{c})
		}
		return
	}

	if lflag&ECHOCTL != 0 && c < 0x20 && c != '\n' && c != '\t' {
		tty.output([]byte{'^', c + 0x40})
		return
	}

	if c == 0x7f && lflag&ECHOCTL != 0 {
		tty.output([]byte("^?"))
		return
	}

	tty.output([]byte{c})
}

func (tty *TTY) erase() {
	if len(tty.line) == 0 {
		return
	}

	// remove a whole UTF-8 sequence
	i := len(tty.line) - 1

	for i > 0 && tty.line[i]&0xc0 == 0x80 {
		i--
	}

	tty.line = tty.line[:i]

	if tty.termios.Lflag&ECHO != 0 && tty.termios.Lflag&ECHOE != 0 {
		tty.output([]byte("\b \b"))
	}
}

// receive processes an input character.
func (tty *TTY) receive(c byte) {
	ios := &tty.termios

	switch {
	case c == '\r' && ios.Iflag&IGNCR != 0:
		return
	case c == '\r' && ios.Iflag&ICRNL != 0:
		c = '\n'
	case c == '\n' && ios.Iflag&INLCR != 0:
		c = '\r'
	}

	if ios.Lflag&ISIG != 0 {
		sig := 0

		switch c {
		case ios.Cc[VINTR]:
			sig = SIGINT
		case ios.Cc[VQUIT]:
			sig = SIGQUIT
		case ios.Cc[VSUSP]:
			sig = SIGTSTP
		}

		if sig != 0 && c != 0 {
			if ios.Lflag&NOFLSH == 0 {
				tty.input = nil
				tty.line = nil
			}

			tty.echo(c)

			if tty.pgrp > 0 {
				signalPgrp(tty.pgrp, sig)
			}

			return
		}
	}

	if ios.Lflag&ICANON == 0 {
		tty.input = append(tty.input, c)
		tty.echo(c)
		return
	}

	switch {
	case c == ios.Cc[VERASE] || c == '\b':
		tty.erase()
	case c == ios.Cc[VWERASE] && ios.Lflag&IEXTEN != 0:
		for len(tty.line) > 0 && tty.line[len(tty.line)-1] == ' ' {
			tty.erase()
		}
		for len(tty.line) > 0 && tty.line[len(tty.line)-1] != ' ' {
			tty.erase()
		}
	case c == ios.Cc[VKILL]:
		for len(tty.line) > 0 {
			tty.erase()
		}
	case c == ios.Cc[VEOF]:
		if len(tty.line) == 0 {
			tty.eof++
		}
		tty.input = append(tty.input, tty.line...)
		tty.line = nil
	case c == '\n' || (c == ios.Cc[VEOL] && c != 0) || (c == ios.Cc[VEOL2] && c != 0):
		tty.line = append(tty.line, c)
		tty.input = append(tty.input, tty.line...)
		tty.line = nil

		if ios.Lflag&ECHO != 0 || ios.Lflag&ECHONL != 0 {
			tty.output([]byte{'\n'})
		}
	default:
		tty.line = append(tty.line, c)
		tty.echo(c)
	}
}

func (tty *TTY) readable() bool {
	return len(tty.input) > 0 || tty.eof > 0 || tty.hungup
}

func (tty *TTY) read(t *Task, f *File, buf []byte) (int, Errno) {
	if len(buf) == 0 {
		return 0, 0
	}

	ios := &tty.termios
	var deadline int64

	min := 1

	if !tty.canonical() {
		vmin := int(ios.Cc[VMIN])
		vtime := int64(ios.Cc[VTIME]) * int64(100*time.Millisecond)

		switch {
		case vmin == 0 && vtime == 0:
			n := copy(buf, tty.input)
			tty.input = tty.input[n:]
			return n, 0
		case vtime > 0:
			deadline = nanotime() + vtime
			if vmin == 0 {
				min = 1
			}
		default:
			min = vmin
		}

		if min > len(buf) {
			min = len(buf)
		}
	}

	ready := func() bool {
		if tty.canonical() {
			return tty.readable()
		}
		return len(tty.input) >= min
	}

	if !ready() {
		if f.nonblock() {
			return 0, EAGAIN
		}

		err := t.block(ready, deadline, true)

		if err == ETIMEDOUT {
			err = 0
		}

		if err != 0 {
			return 0, err
		}
	}

	if len(tty.input) == 0 && tty.hungup {
		return 0, 0
	}

	if tty.canonical() {
		if len(tty.input) == 0 && tty.eof > 0 {
			tty.eof--
			return 0, 0
		}

		// return at most one line
		n := 0

		for n < len(tty.input) && n < len(buf) {
			buf[n] = tty.input[n]
			n++

			if buf[n-1] == '\n' {
				break
			}
		}

		tty.input = tty.input[n:]

		return n, 0
	}

	n := copy(buf, tty.input)
	tty.input = tty.input[n:]

	return n, 0
}

func (tty *TTY) ioctl(t *Task, f *File, cmd uint64, arg uint64) (uint64, Errno) {
	mm := t.proc.mm
	p := t.proc

	switch cmd {
	case TCGETS:
		return 0, mm.copyOut(arg, tty.termios.Bytes())
	case TCSETS, TCSETSW, TCSETSF:
		b := make([]byte, binary.Size(termios{}))

		if err := mm.copyIn(arg, b); err != 0 {
			return 0, err
		}

		wasCanon := tty.canonical()
		tty.termios.unmarshal(b)

		if cmd == TCSETSF {
			tty.input = nil
			tty.line = nil
		}

		if wasCanon && !tty.canonical() {
			// pending line becomes available
			tty.input = append(tty.input, tty.line...)
			tty.line = nil
		}

		wakeAll()

		return 0, 0
	case TIOCGWINSZ:
		ws := &winsize{Row: tty.rows, Col: tty.cols}
		return 0, mm.copyOut(arg, ws.Bytes())
	case TIOCSWINSZ:
		var ws winsize
		b := make([]byte, binary.Size(ws))

		if err := mm.copyIn(arg, b); err != 0 {
			return 0, err
		}

		binary.Decode(b, binary.LittleEndian, &ws)
		tty.rows, tty.cols = ws.Row, ws.Col

		if tty.pgrp > 0 {
			signalPgrp(tty.pgrp, SIGWINCH)
		}

		return 0, 0
	case TIOCGPGRP:
		return 0, mm.writeU32(arg, uint32(tty.pgrp))
	case TIOCSPGRP:
		pgrp, err := mm.readU32(arg)

		if err != 0 {
			return 0, err
		}

		if tty.sid != 0 && p.sid != tty.sid {
			return 0, ENOTTY
		}

		tty.pgrp = int(pgrp)

		return 0, 0
	case TIOCGSID:
		if tty.sid == 0 {
			return 0, ENOTTY
		}
		return 0, mm.writeU32(arg, uint32(tty.sid))
	case TIOCSCTTY:
		if p.sid == p.pid {
			tty.sid = p.sid
			tty.pgrp = p.pgid
		}
		return 0, 0
	case TIOCNOTTY:
		return 0, 0
	case FIONREAD:
		return 0, mm.writeU32(arg, uint32(len(tty.input)))
	case TIOCOUTQ:
		return 0, mm.writeU32(arg, 0)
	case TCSBRK, TCXONC:
		return 0, 0
	case TCFLSH:
		if arg == 0 || arg == 2 {
			tty.input = nil
			tty.line = nil
		}
		return 0, 0
	}

	return 0, EINVAL
}

// ttyFile implements the console character device.
type ttyFile struct {
	tty *TTY
}

func openConsole(flags int) (*File, Errno) {
	return newFile(&ttyFile{tty: console}, consoleInode, flags, "/dev/console"), 0
}

func (d *ttyFile) Read(t *Task, f *File, buf []byte) (int, Errno) {
	return d.tty.read(t, f, buf)
}

func (d *ttyFile) Write(t *Task, f *File, buf []byte) (int, Errno) {
	tty := d.tty

	// pseudo terminal flow control
	if tty.master {
		ready := func() bool { return len(tty.out) < ptyBufSize || tty.hungup }

		if !ready() {
			if f.nonblock() {
				return 0, EAGAIN
			}

			if err := t.block(ready, 0, true); err != 0 {
				return 0, err
			}
		}
	}

	if tty.hungup {
		return 0, EIO
	}

	tty.output(buf)

	return len(buf), 0
}

func (d *ttyFile) Poll(f *File) uint32 {
	var ev uint32

	if !d.tty.master || len(d.tty.out) < ptyBufSize {
		ev |= POLLOUT
	}

	if d.tty.readable() {
		ev |= POLLIN
	}

	if d.tty.hungup {
		ev |= POLLHUP
	}

	return ev
}

func (d *ttyFile) Ioctl(t *Task, f *File, cmd uint64, arg uint64) (uint64, Errno) {
	return d.tty.ioctl(t, f, cmd, arg)
}
