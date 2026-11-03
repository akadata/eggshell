// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

// Regular file contents are stored in pages of the process page pool (see
// mem.go), as in Linux tmpfs, rather than in Go runtime memory: files can
// use the whole pool, a full pool fails writes with ENOSPC and statfs(2) and
// /proc/meminfo account for them.
//
// A file can have read-only base contents (the rootfs archive or synthetic
// contents), each page is copied to the pool when first written.

// fileData represents regular file contents.
type fileData struct {
	size int64

	// read-only initial contents, never longer than size
	base []byte

	// pool page for each file page, 0 for none (base contents or zeros),
	// bytes past size are always zero
	pages []uint64
}

// pool pages holding file data and pool pages reserved to processes (file
// writes fail with ENOSPC once only these are free)
var (
	filePages   uint64
	fileReserve uint64
)

func (d *fileData) setBase(b []byte) {
	d.release()
	d.base = b
	d.size = int64(len(b))
}

func (d *fileData) page(i int64) uint64 {
	if i < int64(len(d.pages)) {
		return d.pages[i]
	}

	return 0
}

// readAt reads file contents at the argument offset.
func (d *fileData) readAt(buf []byte, off int64) (n int) {
	if off < 0 || off >= d.size {
		return 0
	}

	if rem := d.size - off; int64(len(buf)) > rem {
		buf = buf[:rem]
	}

	for n < len(buf) {
		pos := off + int64(n)
		po := int(pos & PageMask)
		b := buf[n:min(len(buf), n+PageSize-po)]

		if pa := d.page(pos >> PageShift); pa != 0 {
			copy(b, physPage(pa)[po:])
		} else {
			c := 0

			if pos < int64(len(d.base)) {
				c = copy(b, d.base[pos:])
			}

			clear(b[c:])
		}

		n += len(b)
	}

	return
}

// bytes returns a copy of up to n bytes at the argument offset.
func (d *fileData) bytes(off int64, n int) []byte {
	if off < 0 || off >= d.size {
		return nil
	}

	b := make([]byte, min(int64(n), d.size-off))
	d.readAt(b, off)

	return b
}

// writeAt writes file contents at the argument offset, extending the file
// as needed, it fails with ENOSPC when out of pages.
func (d *fileData) writeAt(buf []byte, off int64) (n int, err Errno) {
	for n < len(buf) {
		pos := off + int64(n)
		po := int(pos & PageMask)
		c := min(len(buf)-n, PageSize-po)

		pa, e := d.writablePage(pos >> PageShift)

		if e != 0 {
			err = e
			break
		}

		copy(physPage(pa)[po:], buf[n:n+c])
		n += c
	}

	if end := off + int64(n); end > d.size {
		d.size = end
	}

	if n > 0 {
		err = 0
	}

	return
}

// writablePage returns the pool page for the argument file page, allocating
// it when needed.
func (d *fileData) writablePage(i int64) (uint64, Errno) {
	if pa := d.page(i); pa != 0 {
		return pa, 0
	}

	if uint64(len(pages.free)) <= fileReserve {
		return 0, ENOSPC
	}

	pa, ok := allocPage()

	if !ok {
		return 0, ENOSPC
	}

	if off := i << PageShift; off < int64(len(d.base)) {
		copy(physPage(pa), d.base[off:])
	}

	if i >= int64(len(d.pages)) {
		d.pages = append(d.pages, make([]uint64, i+1-int64(len(d.pages)))...)
	}

	d.pages[i] = pa
	filePages++

	return pa, 0
}

// truncate sets the file size, pages past it are released.
func (d *fileData) truncate(size int64) {
	if size < d.size {
		if size < int64(len(d.base)) {
			d.base = d.base[:size:size]
		}

		keep := (size + PageMask) >> PageShift

		for i := keep; i < int64(len(d.pages)); i++ {
			if pa := d.pages[i]; pa != 0 {
				putPage(pa)
				filePages--
			}
		}

		if keep == 0 {
			d.pages = nil
		} else if keep < int64(len(d.pages)) {
			d.pages = d.pages[:keep]
		}

		// zero the tail of the last page
		if po := size & PageMask; po != 0 {
			if pa := d.page(size >> PageShift); pa != 0 {
				clear(physPage(pa)[po:])
			}
		}
	}

	d.size = size
}

// release frees all file contents.
func (d *fileData) release() {
	d.truncate(0)
	d.base = nil
}

// ref and unref track references to an inode from open files and memory
// mappings, its contents are released once unreferenced and unlinked.
func (ino *Inode) ref() {
	ino.refs++
}

func (ino *Inode) unref() {
	ino.refs--
	ino.maybeRelease()
}

func (ino *Inode) maybeRelease() {
	if ino.nlink == 0 && ino.refs == 0 {
		ino.data.release()
	}
}

// contentInode returns an unlinked inode with the argument contents, used to
// map synthetic files.
func contentInode(b []byte) *Inode {
	ino := &Inode{mode: S_IFREG | 0444}
	ino.data.setBase(b)

	return ino
}
