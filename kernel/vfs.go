// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"sort"
	"strings"
	"time"
)

// file mode bits
const (
	S_IFMT   = 0170000
	S_IFSOCK = 0140000
	S_IFLNK  = 0120000
	S_IFREG  = 0100000
	S_IFBLK  = 0060000
	S_IFDIR  = 0040000
	S_IFCHR  = 0020000
	S_IFIFO  = 0010000
)

const (
	AT_FDCWD            = -100
	AT_SYMLINK_NOFOLLOW = 0x100
	AT_REMOVEDIR        = 0x200
	AT_SYMLINK_FOLLOW   = 0x400
	AT_EMPTY_PATH       = 0x1000

	maxSymlinks = 40
)

// Inode represents a file system object.
type Inode struct {
	ino  uint64
	dev  uint64
	mode uint32
	uid  uint32
	gid  uint32

	nlink uint32
	rdev  uint64

	atime, mtime, ctime int64

	// regular files, refs counts open files and memory mappings
	data fileData
	refs int

	// directories
	parent   *Inode
	name     string
	children map[string]*Inode

	// symbolic links
	target string

	// character devices
	device Device

	// named pipes
	fifo *Pipe

	// bound unix domain sockets
	unix *unixListener

	// synthetic (procfs) behaviour
	lookupFn   func(name string) *Inode
	listFn     func() []string
	contentFn  func() []byte
	readlinkFn func() string
}

var (
	rootInode *Inode
	nextIno   uint64 = 1
)

func now() int64 {
	return time.Now().UnixNano()
}

func newInode(mode uint32) *Inode {
	t := now()
	ino := &Inode{
		ino:   nextIno,
		dev:   1,
		mode:  mode,
		nlink: 1,
		atime: t,
		mtime: t,
		ctime: t,
	}
	nextIno++

	if mode&S_IFMT == S_IFDIR {
		ino.children = make(map[string]*Inode)
		ino.nlink = 2
	}

	return ino
}

func (ino *Inode) isDir() bool {
	return ino.mode&S_IFMT == S_IFDIR
}

func (ino *Inode) isReg() bool {
	return ino.mode&S_IFMT == S_IFREG
}

func (ino *Inode) isLink() bool {
	return ino.mode&S_IFMT == S_IFLNK
}

func (ino *Inode) size() int64 {
	switch ino.mode & S_IFMT {
	case S_IFREG:
		if ino.contentFn != nil {
			return 0
		}
		return ino.data.size
	case S_IFLNK:
		return int64(len(ino.linkTarget()))
	case S_IFDIR:
		return 4096
	}
	return 0
}

func (ino *Inode) linkTarget() string {
	if ino.readlinkFn != nil {
		return ino.readlinkFn()
	}
	return ino.target
}

// child returns a directory entry.
func (dir *Inode) child(name string) *Inode {
	switch name {
	case ".":
		return dir
	case "..":
		if dir.parent == nil {
			return dir
		}
		return dir.parent
	}

	if dir.lookupFn != nil {
		return dir.lookupFn(name)
	}

	return dir.children[name]
}

// names returns sorted directory entries.
func (dir *Inode) names() []string {
	if dir.listFn != nil {
		return dir.listFn()
	}

	names := make([]string, 0, len(dir.children))

	for name := range dir.children {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func (dir *Inode) link(name string, ino *Inode) {
	dir.children[name] = ino
	dir.mtime = now()

	if ino.isDir() {
		ino.parent = dir
		ino.name = name
		dir.nlink++
	} else {
		ino.nlink++
	}
}

func (dir *Inode) unlink(name string) {
	ino := dir.children[name]

	if ino == nil {
		return
	}

	delete(dir.children, name)
	dir.mtime = now()

	if ino.isDir() {
		dir.nlink--
		ino.nlink = 0
	} else {
		ino.nlink--
	}

	ino.ctime = now()
	ino.maybeRelease()
}

func initVFS() {
	rootInode = newInode(S_IFDIR | 0755)
	rootInode.name = "/"
}

// mkdirAll creates a directory hierarchy from the root (used at boot).
func mkdirAll(p string, mode uint32) *Inode {
	dir := rootInode

	for c := range strings.SplitSeq(p, "/") {
		if c == "" {
			continue
		}

		next := dir.children[c]

		if next == nil {
			next = newInode(S_IFDIR | mode)
			next.nlink = 1
			dir.link(c, next)
		}

		dir = next
	}

	return dir
}

// startDir returns the starting directory for a relative lookup.
func (t *Task) startDir(dirfd int, p string) (*Inode, Errno) {
	if strings.HasPrefix(p, "/") {
		return rootInode, 0
	}

	if dirfd == AT_FDCWD {
		return t.proc.fs.cwd, 0
	}

	f := t.proc.files.get(dirfd)

	if f == nil {
		return nil, EBADF
	}

	if f.inode == nil || !f.inode.isDir() {
		return nil, ENOTDIR
	}

	return f.inode, 0
}

// walk resolves a path, following symbolic links for the last component
// if follow is set.
func (t *Task) walk(dir *Inode, p string, follow bool, depth *int) (*Inode, Errno) {
	if p == "" {
		return nil, ENOENT
	}

	if strings.HasPrefix(p, "/") {
		dir = rootInode
	}

	comps := strings.Split(p, "/")

	// trailing slash requires a directory
	trailing := strings.HasSuffix(p, "/")

	cur := dir

	for i, c := range comps {
		if c == "" || c == "." {
			continue
		}

		if !cur.isDir() {
			return nil, ENOTDIR
		}

		if len(c) > 255 {
			return nil, ENAMETOOLONG
		}

		next := cur.child(c)

		if next == nil {
			return nil, ENOENT
		}

		last := true

		for _, r := range comps[i+1:] {
			if r != "" && r != "." {
				last = false
				break
			}
		}

		if next.isLink() && (!last || follow || trailing) {
			*depth++

			if *depth > maxSymlinks {
				return nil, ELOOP
			}

			target, err := t.walk(cur, next.linkTarget(), true, depth)

			if err != 0 {
				return nil, err
			}

			next = target
		}

		cur = next
	}

	if trailing && !cur.isDir() {
		return nil, ENOTDIR
	}

	return cur, 0
}

// lookup resolves a path relative to the argument directory descriptor.
func (t *Task) lookup(dirfd int, p string, follow bool) (*Inode, Errno) {
	dir, err := t.startDir(dirfd, p)

	if err != 0 {
		return nil, err
	}

	depth := 0

	return t.walk(dir, p, follow, &depth)
}

// lookupAt resolves a path honoring AT_EMPTY_PATH and AT_SYMLINK_NOFOLLOW.
func (t *Task) lookupAt(dirfd int, p string, flags int) (*Inode, Errno) {
	if p == "" && flags&AT_EMPTY_PATH != 0 {
		if dirfd == AT_FDCWD {
			return t.proc.fs.cwd, 0
		}

		f := t.proc.files.get(dirfd)

		if f == nil {
			return nil, EBADF
		}

		if f.inode == nil {
			return f.anonInode(), 0
		}

		return f.inode, 0
	}

	return t.lookup(dirfd, p, flags&AT_SYMLINK_NOFOLLOW == 0)
}

// lookupParent resolves the parent directory of a path, returning the last
// component name.
func (t *Task) lookupParent(dirfd int, p string) (dir *Inode, name string, err Errno) {
	if p == "" {
		return nil, "", ENOENT
	}

	trimmed := strings.TrimRight(p, "/")

	if trimmed == "" {
		// root
		return rootInode, ".", 0
	}

	start, err := t.startDir(dirfd, p)

	if err != 0 {
		return
	}

	i := strings.LastIndexByte(trimmed, '/')

	var parentPath string

	switch {
	case i < 0:
		parentPath = "."
		name = trimmed
	case i == 0:
		parentPath = "/"
		name = trimmed[1:]
	default:
		parentPath = trimmed[:i]
		name = trimmed[i+1:]
	}

	if len(name) > 255 {
		return nil, "", ENAMETOOLONG
	}

	depth := 0

	if dir, err = t.walk(start, parentPath, true, &depth); err != 0 {
		return
	}

	if !dir.isDir() {
		return nil, "", ENOTDIR
	}

	return
}

// inodePath returns the absolute path of a directory inode.
func inodePath(dir *Inode) string {
	if dir == rootInode {
		return "/"
	}

	var parts []string

	for d := dir; d != nil && d != rootInode; d = d.parent {
		parts = append([]string{d.name}, parts...)
	}

	return "/" + strings.Join(parts, "/")
}

// absPath returns the absolute (lexical) form of a path.
func (t *Task) absPath(dirfd int, p string) (string, Errno) {
	if strings.HasPrefix(p, "/") {
		return cleanPath(p), 0
	}

	dir, err := t.startDir(dirfd, p)

	if err != 0 {
		return "", err
	}

	return cleanPath(inodePath(dir) + "/" + p), 0
}

func cleanPath(p string) string {
	var out []string

	for c := range strings.SplitSeq(p, "/") {
		switch c {
		case "", ".":
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, c)
		}
	}

	return "/" + strings.Join(out, "/")
}
