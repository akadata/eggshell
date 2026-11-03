// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

func registerMMSyscalls() {
	reg(SYS_BRK, func(t *Task, a *[6]uint64) (uint64, Errno) {
		return t.proc.mm.setBrk(a[0]), 0
	})

	reg(SYS_MMAP, func(t *Task, a *[6]uint64) (uint64, Errno) {
		addr, length, prot, flags, fd, off := a[0], a[1], int(a[2]), int(a[3]), int(int32(a[4])), a[5]
		mm := t.proc.mm

		if off&PageMask != 0 {
			return fail(EINVAL)
		}

		if flags&(MAP_SHARED|MAP_PRIVATE) == 0 {
			return fail(EINVAL)
		}

		var file, backing *Inode
		name := ""

		if flags&MAP_ANONYMOUS == 0 {
			f := t.proc.files.get(fd)

			if f == nil {
				return fail(EBADF)
			}

			ino := f.inode

			switch {
			case ino == nil:
				return fail(ENODEV)
			case ino.mode&S_IFMT == S_IFCHR:
				// /dev/zero and friends: anonymous memory
			case ino.isReg():
				if !f.readable() {
					return fail(EACCES)
				}

				file = ino

				if f.content != nil {
					file = contentInode(f.content)
				}

				// shared writable mapping of a regular file
				if flags&MAP_SHARED != 0 && prot&PROT_WRITE != 0 && f.content == nil {
					if !f.writable() {
						return fail(EACCES)
					}

					backing = ino
				}

				name = f.path
			default:
				return fail(ENODEV)
			}
		}

		if flags&MAP_STACK != 0 && name == "" {
			name = "[stack]"
		}

		r, err := mm.mmap(addr, length, prot, flags, file, backing, off, name)

		return r, err
	})

	reg(SYS_MUNMAP, func(t *Task, a *[6]uint64) (uint64, Errno) {
		return 0, t.proc.mm.munmap(a[0], a[1])
	})

	reg(SYS_MPROTECT, func(t *Task, a *[6]uint64) (uint64, Errno) {
		return 0, t.proc.mm.mprotect(a[0], a[1], int(a[2]))
	})

	reg(SYS_MREMAP, func(t *Task, a *[6]uint64) (uint64, Errno) {
		return t.proc.mm.mremap(a[0], a[1], a[2], int(a[3]), a[4])
	})

	reg(SYS_MADVISE, func(t *Task, a *[6]uint64) (uint64, Errno) {
		const (
			MADV_DONTNEED = 4
			MADV_FREE     = 8
		)

		mm := t.proc.mm

		if a[0]&PageMask != 0 {
			return fail(EINVAL)
		}

		switch a[2] {
		case MADV_DONTNEED, MADV_FREE:
			end := pageUp(a[0] + a[1])

			// only private mappings drop their pages
			for va := a[0]; va < end; {
				v := mm.findVMA(va)

				if v == nil {
					va = pageUp(va + 1)
					continue
				}

				e := min(v.end, end)

				if !v.shared {
					mm.unmapPages(va, e)
				}

				va = e
			}
		}

		return ok(0)
	})

	reg(SYS_MSYNC, func(t *Task, a *[6]uint64) (uint64, Errno) {
		if a[0]&PageMask != 0 {
			return fail(EINVAL)
		}

		t.proc.mm.writeBack(a[0], pageUp(a[0]+a[1]))

		return ok(0)
	})

	for _, n := range []uint64{SYS_MLOCK, SYS_MUNLOCK, SYS_MLOCKALL, SYS_MUNLOCKALL, SYS_MLOCK2} {
		reg(n, func(t *Task, a *[6]uint64) (uint64, Errno) { return ok(0) })
	}

	reg(SYS_MINCORE, func(t *Task, a *[6]uint64) (uint64, Errno) {
		n := pageUp(a[1]) / PageSize
		b := make([]byte, n)

		for i := range b {
			b[i] = 1
		}

		return uint64(int64(0)), t.proc.mm.copyOut(a[2], b)
	})
}
