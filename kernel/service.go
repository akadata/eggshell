// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/netip"
	"time"
)

// This file provides kernel services, Go code running alongside processes
// such as the SSH server, with access to the network stack, file system and
// processes. All functions are safe to call without holding the kernel lock
// once the kernel is started (see [Config.Ready]).

// ktask is the context used by kernel services for blocking operations on
// kernel files, it has no address space and ignores SIGPIPE.
var ktask *Task

func kernelTask() *Task {
	if ktask == nil {
		p := &Proc{
			files:   newFDTable(),
			fs:      &FSContext{cwd: rootInode, umask: 022},
			sigh:    &SigHand{},
			done:    make(chan struct{}),
			rlimits: defaultRlimits(),
		}

		p.sigh.act[SIGPIPE].Handler = SIG_IGN
		ktask = &Task{proc: p}
	}

	return ktask
}

// kfile provides access to a kernel file (socket or pipe end), it is
// released once closed and no longer in use.
type kfile struct {
	f      *File
	busy   int
	closed bool
}

func (k *kfile) do(fn func(t *Task, f *File) (int, Errno)) (int, error) {
	klock.Lock()
	defer klock.Unlock()

	if k.closed {
		return 0, net.ErrClosed
	}

	k.busy++
	n, err := fn(kernelTask(), k.f)
	k.busy--
	k.release()

	if err != 0 {
		return n, err
	}

	return n, nil
}

func (k *kfile) release() {
	if k.closed && k.busy == 0 && k.f != nil {
		k.f.decref()
		k.f = nil
		wakeAll()
	}
}

func (k *kfile) Read(buf []byte) (n int, err error) {
	n, err = k.do(func(t *Task, f *File) (int, Errno) {
		return f.ops.Read(t, f, buf)
	})

	if n == 0 && err == nil && len(buf) > 0 {
		err = io.EOF
	}

	return
}

func (k *kfile) Write(buf []byte) (n int, err error) {
	for n < len(buf) && err == nil {
		var w int

		w, err = k.do(func(t *Task, f *File) (int, Errno) {
			return f.ops.Write(t, f, buf[n:])
		})

		n += w
	}

	return
}

func (k *kfile) Close() error {
	klock.Lock()
	defer klock.Unlock()

	if k.closed {
		return nil
	}

	k.closed = true

	// wake up blocked readers and writers
	if s, ok := k.f.ops.(socket); ok {
		s.shutdown(SHUT_RDWR)
	}

	wakeAll()
	k.release()

	return nil
}

// tcpListener implements [net.Listener] on a kernel socket.
type tcpListener struct {
	kfile
	addr *net.TCPAddr
}

// Listen announces on the argument TCP port of all local addresses.
func Listen(port uint16) (net.Listener, error) {
	klock.Lock()
	defer klock.Unlock()

	t := kernelTask()
	s, err := inetSocketCreate(SOCK_STREAM, 0)

	if err != 0 {
		return nil, err
	}

	f := newFile(s, nil, O_RDWR, "socket:")

	// struct sockaddr_in, INADDR_ANY
	addr := make([]byte, 16)
	binary.LittleEndian.PutUint16(addr[0:], AF_INET)
	binary.BigEndian.PutUint16(addr[2:], port)

	if err = s.bind(t, addr); err == 0 {
		err = s.listen(t, 16)
	}

	if err != 0 {
		f.decref()
		return nil, err
	}

	return &tcpListener{
		f:    f,
		addr: &net.TCPAddr{IP: net.IPv4zero, Port: int(port)},
	}, nil
}

func (l *tcpListener) Accept() (net.Conn, error) {
	var c *tcpConn

	_, err := l.do(func(t *Task, f *File) (int, Errno) {
		nf, peer, err := f.ops.(socket).accept(t, f)

		if err == 0 {
			c = &tcpConn{f: nf, local: l.addr, remote: sockaddrTCP(peer)}
		}

		return 0, err
	})

	if err != nil {
		return nil, err
	}

	return c, nil
}

func (l *tcpListener) Addr() net.Addr {
	return l.addr
}

// tcpConn implements [net.Conn] on a kernel socket, deadlines are not
// supported.
type tcpConn struct {
	kfile
	local  net.Addr
	remote net.Addr
}

func (c *tcpConn) LocalAddr() net.Addr {
	return c.local
}

func (c *tcpConn) RemoteAddr() net.Addr {
	return c.remote
}

func (c *tcpConn) SetDeadline(_ time.Time) error {
	return errors.ErrUnsupported
}

func (c *tcpConn) SetReadDeadline(_ time.Time) error {
	return errors.ErrUnsupported
}

func (c *tcpConn) SetWriteDeadline(_ time.Time) error {
	return errors.ErrUnsupported
}

func sockaddrTCP(sa []byte) *net.TCPAddr {
	if len(sa) < 8 {
		return &net.TCPAddr{}
	}

	ip := netip.AddrFrom4([4]byte(sa[4:8]))
	port := binary.BigEndian.Uint16(sa[2:])

	return net.TCPAddrFromAddrPort(netip.AddrPortFrom(ip, port))
}

// ReadFile returns the content of a regular file.
func ReadFile(path string) ([]byte, error) {
	klock.Lock()
	defer klock.Unlock()

	ino, err := kernelTask().lookup(AT_FDCWD, path, true)

	if err != 0 {
		return nil, err
	}

	if !ino.isReg() {
		return nil, EINVAL
	}

	return ino.data.bytes(0, int(ino.data.size)), nil
}

// Cmd represents a process to start with [Exec].
type Cmd struct {
	// Args holds the command line, Args[0] is the executable path.
	Args []string
	// Env holds the environment.
	Env []string
	// Dir is the working directory, the root directory if empty.
	Dir string

	// PTY, if not nil, is the controlling terminal and standard streams
	// of the process, otherwise these are pipes (see [Process]).
	PTY *PTY
}

// Process represents a process started with [Exec], leader of its own
// session.
type Process struct {
	// Stdin, Stdout and Stderr are the process standard streams, when not
	// started on a pseudo terminal.
	Stdin  io.WriteCloser
	Stdout io.ReadCloser
	Stderr io.ReadCloser

	p *Proc
}

// Exec starts a process.
func Exec(c *Cmd) (proc *Process, err error) {
	klock.Lock()
	defer klock.Unlock()

	if len(c.Args) == 0 {
		return nil, EINVAL
	}

	cwd := rootInode

	if c.Dir != "" {
		ino, e := kernelTask().lookup(AT_FDCWD, c.Dir, true)

		if e == 0 && ino.isDir() {
			cwd = ino
		}
	}

	proc = &Process{}

	var stdio [3]*File
	var tty *TTY

	if c.PTY != nil {
		tty = c.PTY.tty

		if tty.hungup {
			return nil, EIO
		}

		for i := range stdio {
			stdio[i] = c.PTY.open()
		}
	} else {
		r, w := newPipe()
		stdio[0], proc.Stdin = r, &kfile{f: w}

		r, w = newPipe()
		stdio[1], proc.Stdout = w, &kfile{f: r}

		r, w = newPipe()
		stdio[2], proc.Stderr = w, &kfile{f: r}
	}

	if proc.p, err = spawn(c.Args, c.Env, cwd, stdio, tty); err != nil {
		for _, f := range []io.Closer{proc.Stdin, proc.Stdout, proc.Stderr} {
			if f != nil {
				f.(*kfile).closed = true
				f.(*kfile).release()
			}
		}

		return nil, err
	}

	return
}

// Wait waits for the process to exit, it returns its exit code, or the
// signal which terminated it.
func (proc *Process) Wait() (code int, sig int) {
	<-proc.p.done

	klock.Lock()
	defer klock.Unlock()

	status := proc.p.exitStatus

	return (status >> 8) & 0xff, status & 0x7f
}

// Signal sends a signal to the process.
func (proc *Process) Signal(sig int) {
	klock.Lock()
	defer klock.Unlock()

	proc.p.signal(sig, SigInfo{Signo: sig, Code: SI_KERNEL})
}
