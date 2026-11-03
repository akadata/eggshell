// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

const (
	pipeSize = 65536
	pipeBuf  = 4096
)

// Pipe represents a unidirectional byte stream.
type Pipe struct {
	// ring buffer of pipeSize bytes, held only while data is buffered
	ring  []byte
	ringp *[]byte
	head  int
	n     int

	readers int
	writers int
}

// peek copies buffered data without consuming it.
func (p *Pipe) peek(b []byte) int {
	n := min(len(b), p.n)

	if n == 0 {
		return 0
	}

	c := copy(b[:n], p.ring[p.head:])
	copy(b[c:n], p.ring)

	return n
}

// read consumes buffered data.
func (p *Pipe) read(b []byte) int {
	n := p.peek(b)

	p.head = (p.head + n) % pipeSize
	p.n -= n

	if p.n == 0 {
		putIOBuf(p.ringp)
		p.ring, p.ringp = nil, nil
		p.head = 0
	}

	return n
}

// write buffers data up to the free space.
func (p *Pipe) write(b []byte) int {
	n := min(len(b), pipeSize-p.n)

	if n == 0 {
		return 0
	}

	if p.ring == nil {
		p.ring, p.ringp = getIOBuf(pipeSize)
	}

	tail := (p.head + p.n) % pipeSize
	c := copy(p.ring[tail:], b[:n])
	copy(p.ring, b[c:n])
	p.n += n

	return n
}

type pipeEnd struct {
	pipe  *Pipe
	write bool
}

func (e *pipeEnd) Mode() uint32 {
	return S_IFIFO | 0600
}

func (e *pipeEnd) Read(t *Task, f *File, buf []byte) (int, Errno) {
	p := e.pipe

	if e.write {
		return 0, EBADF
	}

	if len(buf) == 0 {
		return 0, 0
	}

	ready := func() bool { return p.n > 0 || p.writers == 0 }

	if !ready() {
		if f.nonblock() {
			return 0, EAGAIN
		}

		if err := t.block(ready, 0, true); err != 0 {
			return 0, err
		}
	}

	n := p.read(buf)

	wakeAll()

	return n, 0
}

func (e *pipeEnd) Write(t *Task, f *File, buf []byte) (int, Errno) {
	p := e.pipe

	if !e.write {
		return 0, EBADF
	}

	written := 0

	for written < len(buf) {
		if p.readers == 0 {
			t.signal(SIGPIPE, SigInfo{Signo: SIGPIPE})

			if written > 0 {
				return written, 0
			}

			return 0, EPIPE
		}

		rem := len(buf) - written
		space := pipeSize - p.n

		// writes up to PIPE_BUF are atomic
		need := 1

		if rem <= pipeBuf {
			need = rem
		}

		if space < need {
			if f.nonblock() {
				if written > 0 {
					return written, 0
				}

				return 0, EAGAIN
			}

			err := t.block(func() bool {
				return pipeSize-p.n >= need || p.readers == 0
			}, 0, true)

			if err != 0 {
				if written > 0 {
					return written, 0
				}

				return 0, err
			}

			continue
		}

		n := min(rem, space)

		p.write(buf[written : written+n])
		written += n

		wakeAll()
	}

	return written, 0
}

func (e *pipeEnd) Poll(f *File) uint32 {
	p := e.pipe
	var ev uint32

	if e.write {
		if p.readers == 0 {
			ev |= POLLERR
		} else if pipeSize-p.n >= pipeBuf {
			ev |= POLLOUT
		}
	} else {
		if p.n > 0 {
			ev |= POLLIN
		}

		if p.writers == 0 {
			ev |= POLLHUP
		}
	}

	return ev
}

func (e *pipeEnd) Ioctl(t *Task, f *File, cmd uint64, arg uint64) (uint64, Errno) {
	if cmd == FIONREAD {
		return 0, t.proc.mm.writeU32(arg, uint32(e.pipe.n))
	}

	return 0, ENOTTY
}

func (e *pipeEnd) Close(f *File) {
	if e.write {
		e.pipe.writers--
	} else {
		e.pipe.readers--
	}

	wakeAll()
}

func newPipe() (r *File, w *File) {
	p := &Pipe{readers: 1, writers: 1}

	r = newFile(&pipeEnd{pipe: p}, nil, O_RDONLY, "pipe:")
	w = newFile(&pipeEnd{pipe: p, write: true}, nil, O_WRONLY, "pipe:")

	return
}

func openFIFO(t *Task, ino *Inode, flags int, path string) (*File, Errno) {
	if ino.fifo == nil {
		ino.fifo = &Pipe{}
	}

	p := ino.fifo
	acc := flags & O_ACCMODE

	var f *File

	switch acc {
	case O_RDONLY:
		p.readers++
		f = newFile(&pipeEnd{pipe: p}, ino, flags, path)
	case O_WRONLY:
		if p.readers == 0 && flags&O_NONBLOCK != 0 {
			return nil, ENXIO
		}
		p.writers++
		f = newFile(&pipeEnd{pipe: p, write: true}, ino, flags, path)
	default:
		return nil, EINVAL
	}

	f.anon = ino
	wakeAll()

	if flags&O_NONBLOCK == 0 {
		if err := t.block(func() bool {
			return p.readers > 0 && p.writers > 0
		}, 0, true); err != 0 {
			f.decref()
			return nil, err
		}
	}

	return f, 0
}
