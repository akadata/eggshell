// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

// Device represents a character device.
type Device interface {
	Open(t *Task, ino *Inode, flags int, path string) (*File, Errno)
}

type simpleDevice struct {
	ops FileOps
}

func (d simpleDevice) Open(t *Task, ino *Inode, flags int, path string) (*File, Errno) {
	return newFile(d.ops, ino, flags, path), 0
}

type nullDev struct{}

func (nullDev) Read(t *Task, f *File, buf []byte) (int, Errno) {
	return 0, 0
}

func (nullDev) Write(t *Task, f *File, buf []byte) (int, Errno) {
	return len(buf), 0
}

type zeroDev struct{}

func (zeroDev) Read(t *Task, f *File, buf []byte) (int, Errno) {
	clear(buf)
	return len(buf), 0
}
func (zeroDev) Write(t *Task, f *File, buf []byte) (int, Errno) {
	return len(buf), 0
}

type fullDev struct{}

func (fullDev) Read(t *Task, f *File, buf []byte) (int, Errno) {
	clear(buf)
	return len(buf), 0
}

func (fullDev) Write(t *Task, f *File, buf []byte) (int, Errno) {
	return 0, ENOSPC
}

type randomDev struct{}

func (randomDev) Read(t *Task, f *File, buf []byte) (int, Errno) {
	getRandom(buf)
	return len(buf), 0
}

func (randomDev) Write(t *Task, f *File, buf []byte) (int, Errno) {
	return len(buf), 0
}

// console device: /dev/console, /dev/ttyS0
type consoleDev struct{}

func (consoleDev) Open(t *Task, ino *Inode, flags int, path string) (*File, Errno) {
	p := t.proc

	// acquire controlling terminal
	if flags&O_NOCTTY == 0 && p.sid == p.pid && console.sid == 0 {
		console.sid = p.sid
		console.pgrp = p.pgid
	}

	return newFile(&ttyFile{tty: console}, ino, flags, path), 0
}

// /dev/tty: the controlling terminal
type ctlTTYDev struct{}

func (ctlTTYDev) Open(t *Task, ino *Inode, flags int, path string) (*File, Errno) {
	tty := ctty(t.proc)

	if tty == nil {
		return nil, ENXIO
	}

	return newFile(&ttyFile{tty: tty}, ino, flags, path), 0
}

var consoleInode *Inode

func mkdev(dir *Inode, name string, major, minor uint64, dev Device) *Inode {
	ino := newInode(S_IFCHR | 0666)
	ino.rdev = major<<8 | minor
	ino.device = dev
	ino.nlink = 0

	dir.link(name, ino)

	return ino
}

func initDevices() {
	dev := mkdirAll("/dev", 0755)

	mkdev(dev, "null", 1, 3, simpleDevice{nullDev{}})
	mkdev(dev, "zero", 1, 5, simpleDevice{zeroDev{}})
	mkdev(dev, "full", 1, 7, simpleDevice{fullDev{}})
	mkdev(dev, "random", 1, 8, simpleDevice{randomDev{}})
	mkdev(dev, "urandom", 1, 9, simpleDevice{randomDev{}})
	mkdev(dev, "tty", 5, 0, ctlTTYDev{})

	consoleInode = mkdev(dev, "console", 5, 1, consoleDev{})
	consoleInode.mode = S_IFCHR | 0620
	mkdev(dev, "ttyS0", 4, 64, consoleDev{})

	for name, target := range map[string]string{
		"stdin":  "/proc/self/fd/0",
		"stdout": "/proc/self/fd/1",
		"stderr": "/proc/self/fd/2",
		"fd":     "/proc/self/fd",
	} {
		if dev.children[name] == nil {
			l := newInode(S_IFLNK | 0777)
			l.target = target
			l.nlink = 0
			dev.link(name, l)
		}
	}

	mkdirAll("/dev/shm", 01777)
	mkdirAll("/dev/pts", 0755)
	mkdirAll("/tmp", 01777).mode = S_IFDIR | 01777
}
