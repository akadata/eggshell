// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

// open flags
const (
	O_ACCMODE   = 0x3
	O_RDONLY    = 0x0
	O_WRONLY    = 0x1
	O_RDWR      = 0x2
	O_CREAT     = 0x40
	O_EXCL      = 0x80
	O_NOCTTY    = 0x100
	O_TRUNC     = 0x200
	O_APPEND    = 0x400
	O_NONBLOCK  = 0x800
	O_DSYNC     = 0x1000
	O_ASYNC     = 0x2000
	O_DIRECT    = 0x4000
	O_LARGEFILE = 0x8000
	O_DIRECTORY = 0x10000
	O_NOFOLLOW  = 0x20000
	O_NOATIME   = 0x40000
	O_CLOEXEC   = 0x80000
	O_PATH      = 0x200000
	O_TMPFILE   = 0x410000
)

// poll events
const (
	POLLIN     = 0x001
	POLLPRI    = 0x002
	POLLOUT    = 0x004
	POLLERR    = 0x008
	POLLHUP    = 0x010
	POLLNVAL   = 0x020
	POLLRDNORM = 0x040
	POLLWRNORM = 0x100
	POLLRDHUP  = 0x2000
)

const maxFDs = 65536

type FileOps interface {
	Read(t *Task, f *File, buf []byte) (int, Errno)
	Write(t *Task, f *File, buf []byte) (int, Errno)
}

type poller interface {
	Poll(f *File) uint32
}

type ioctler interface {
	Ioctl(t *Task, f *File, cmd uint64, arg uint64) (uint64, Errno)
}

type closer interface {
	Close(f *File)
}

type positional interface {
	ReadAt(t *Task, f *File, buf []byte, off int64) (int, Errno)
	WriteAt(t *Task, f *File, buf []byte, off int64) (int, Errno)
}

type statter interface {
	Mode() uint32
}

// File represents an open file description.
type File struct {
	ops   FileOps
	inode *Inode
	pos   int64
	flags int
	refs  int
	path  string

	// directory listing snapshot
	dirents []string

	// synthetic file contents
	content []byte

	anon *Inode
}

func newFile(ops FileOps, ino *Inode, flags int, path string) *File {
	if ino != nil {
		ino.ref()
	}

	return &File{ops: ops, inode: ino, flags: flags, refs: 1, path: path}
}

func (f *File) incref() {
	f.refs++
}

func (f *File) decref() {
	f.refs--

	if f.refs == 0 {
		if c, ok := f.ops.(closer); ok {
			c.Close(f)
		}

		if f.inode != nil {
			f.inode.unref()
		}
	}
}

func (f *File) readable() bool {
	return f.flags&O_ACCMODE != O_WRONLY && f.flags&O_PATH == 0
}

func (f *File) writable() bool {
	return f.flags&O_ACCMODE != O_RDONLY && f.flags&O_PATH == 0
}

func (f *File) nonblock() bool {
	return f.flags&O_NONBLOCK != 0
}

func (f *File) poll() uint32 {
	if p, ok := f.ops.(poller); ok {
		return p.Poll(f)
	}
	return POLLIN | POLLOUT
}

// anonInode returns an inode describing a file without file system
// representation (pipes, sockets).
func (f *File) anonInode() *Inode {
	if f.anon == nil {
		mode := uint32(S_IFREG | 0600)

		if s, ok := f.ops.(statter); ok {
			mode = s.Mode()
		}

		f.anon = newInode(mode)
		f.anon.dev = 3
	}

	return f.anon
}

type fdEntry struct {
	f       *File
	cloexec bool
}

// FDTable represents a file descriptor table.
type FDTable struct {
	fds  []fdEntry
	refs int
}

func newFDTable() *FDTable {
	return &FDTable{refs: 1}
}

func (t *FDTable) get(fd int) *File {
	if fd < 0 || fd >= len(t.fds) {
		return nil
	}

	return t.fds[fd].f
}

// install adds a file to the table at the lowest descriptor >= min.
func (t *FDTable) install(f *File, min int, cloexec bool) int {
	for fd := min; fd < maxFDs; fd++ {
		if fd >= len(t.fds) {
			t.fds = append(t.fds, make([]fdEntry, fd-len(t.fds)+1)...)
		}

		if t.fds[fd].f == nil {
			f.incref()
			t.fds[fd] = fdEntry{f: f, cloexec: cloexec}
			return fd
		}
	}

	return -int(EMFILE)
}

// installAt places a file at a specific descriptor, closing any previous one.
func (t *FDTable) installAt(f *File, fd int, cloexec bool) {
	if fd >= len(t.fds) {
		t.fds = append(t.fds, make([]fdEntry, fd-len(t.fds)+1)...)
	}

	old := t.fds[fd].f
	f.incref()
	t.fds[fd] = fdEntry{f: f, cloexec: cloexec}

	if old != nil {
		old.decref()
	}
}

func (t *FDTable) close(fd int) Errno {
	f := t.get(fd)

	if f == nil {
		return EBADF
	}

	t.fds[fd] = fdEntry{}
	f.decref()

	return 0
}

func (t *FDTable) dup() *FDTable {
	n := &FDTable{refs: 1, fds: make([]fdEntry, len(t.fds))}

	for i, e := range t.fds {
		if e.f != nil {
			e.f.incref()
			n.fds[i] = e
		}
	}

	return n
}

func (t *FDTable) closeOnExec() {
	for fd, e := range t.fds {
		if e.f != nil && e.cloexec {
			t.fds[fd] = fdEntry{}
			e.f.decref()
		}
	}
}

func (t *FDTable) release() {
	t.refs--

	if t.refs > 0 {
		return
	}

	for fd, e := range t.fds {
		if e.f != nil {
			t.fds[fd] = fdEntry{}
			e.f.decref()
		}
	}
}

// regular file operations
type regularFile struct{}

func (regularFile) Read(t *Task, f *File, buf []byte) (int, Errno) {
	n, err := regularFile{}.ReadAt(t, f, buf, f.pos)
	f.pos += int64(n)
	return n, err
}

func (regularFile) Write(t *Task, f *File, buf []byte) (int, Errno) {
	if f.flags&O_APPEND != 0 {
		f.pos = f.inode.data.size
	}

	n, err := regularFile{}.WriteAt(t, f, buf, f.pos)
	f.pos += int64(n)

	return n, err
}

func (regularFile) ReadAt(t *Task, f *File, buf []byte, off int64) (int, Errno) {
	if f.content != nil {
		if off >= int64(len(f.content)) {
			return 0, 0
		}

		return copy(buf, f.content[off:]), 0
	}

	return f.inode.data.readAt(buf, off), 0
}

func (regularFile) WriteAt(t *Task, f *File, buf []byte, off int64) (int, Errno) {
	ino := f.inode

	if ino.contentFn != nil {
		// synthetic files are read-only, accept writes to tunables
		return len(buf), 0
	}

	if off < 0 {
		return 0, EINVAL
	}

	if off+int64(len(buf)) > 1<<34 {
		return 0, EFBIG
	}

	n, err := ino.data.writeAt(buf, off)

	if n > 0 {
		ino.mtime = now()
		ino.ctime = ino.mtime
	}

	return n, err
}

func truncate(ino *Inode, size int64) Errno {
	if ino.isDir() {
		return EISDIR
	}

	if !ino.isReg() {
		return EINVAL
	}

	if size < 0 {
		return EINVAL
	}

	if size > 1<<34 {
		return EFBIG
	}

	ino.data.truncate(size)
	ino.mtime = now()
	ino.ctime = ino.mtime

	return 0
}

// directory operations
type dirFile struct{}

func (dirFile) Read(t *Task, f *File, buf []byte) (int, Errno) {
	return 0, EISDIR
}

func (dirFile) Write(t *Task, f *File, buf []byte) (int, Errno) {
	return 0, EISDIR
}

// path-only descriptors (O_PATH)
type pathFile struct{}

func (pathFile) Read(t *Task, f *File, buf []byte) (int, Errno) {
	return 0, EBADF
}

func (pathFile) Write(t *Task, f *File, buf []byte) (int, Errno) {
	return 0, EBADF
}

// openInode creates an open file description for an inode.
func (t *Task) openInode(ino *Inode, flags int, path string) (*File, Errno) {
	if flags&O_PATH != 0 {
		return newFile(pathFile{}, ino, flags, path), 0
	}

	switch ino.mode & S_IFMT {
	case S_IFDIR:
		if flags&O_ACCMODE != O_RDONLY {
			return nil, EISDIR
		}
		return newFile(dirFile{}, ino, flags, path), 0
	case S_IFREG:
		f := newFile(regularFile{}, ino, flags, path)

		if ino.contentFn != nil {
			f.content = ino.contentFn()
		} else if flags&O_TRUNC != 0 && flags&O_ACCMODE != O_RDONLY {
			truncate(ino, 0)
		}

		return f, 0
	case S_IFCHR:
		if ino.device == nil {
			return nil, ENXIO
		}
		return ino.device.Open(t, ino, flags, path)
	case S_IFIFO:
		return openFIFO(t, ino, flags, path)
	case S_IFSOCK:
		return nil, ENXIO
	case S_IFLNK:
		return nil, ELOOP
	}

	return nil, ENXIO
}

// open implements openat(2).
func (t *Task) open(dirfd int, p string, flags int, mode uint32) (int, Errno) {
	follow := flags&O_NOFOLLOW == 0

	ino, err := t.lookup(dirfd, p, follow)

	switch {
	case err == ENOENT && flags&O_CREAT != 0:
		dir, name, err := t.lookupParent(dirfd, p)

		if err != 0 {
			return 0, err
		}

		if name == "." || name == ".." || dir.lookupFn != nil {
			return 0, EISDIR
		}

		if dir.nlink == 0 {
			return 0, ENOENT
		}

		ino = newInode(S_IFREG | (mode &^ t.proc.fs.umask & 07777))
		ino.nlink = 0
		dir.link(name, ino)
	case err != 0:
		return 0, err
	default:
		if flags&O_CREAT != 0 && flags&O_EXCL != 0 {
			return 0, EEXIST
		}

		if ino.isLink() {
			return 0, ELOOP
		}
	}

	if flags&O_DIRECTORY != 0 && !ino.isDir() {
		return 0, ENOTDIR
	}

	abs, _ := t.absPath(dirfd, p)

	f, err := t.openInode(ino, flags&^O_CLOEXEC, abs)

	if err != 0 {
		return 0, err
	}

	fd := t.proc.files.install(f, 0, flags&O_CLOEXEC != 0)
	f.decref()

	if fd < 0 {
		return 0, Errno(-fd)
	}

	return fd, 0
}
