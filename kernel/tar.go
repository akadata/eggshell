// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"archive/tar"
	"bytes"
	"io"
	"path"
	"strings"
)

func unpackTar(archive []byte) error {
	br := bytes.NewReader(archive)
	r := tar.NewReader(br)

	type hardlink struct{ name, target string }
	var links []hardlink

	for {
		hdr, err := r.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		name := cleanPath(hdr.Name)

		if name == "/" {
			rootInode.mode = S_IFDIR | uint32(hdr.Mode&07777)
			continue
		}

		dir := mkdirAll(path.Dir(name), 0755)
		base := path.Base(name)
		perm := uint32(hdr.Mode & 07777)

		var ino *Inode

		switch hdr.Typeflag {
		case tar.TypeDir:
			ino = mkdirAll(name, perm)
			ino.mode = S_IFDIR | perm
			ino.mtime = hdr.ModTime.UnixNano()
			continue
		case tar.TypeReg:
			ino = newInode(S_IFREG | perm)

			if hdr.Size > 0 {
				// slice the archive directly to avoid a copy, pages
				// are copied to the page pool when first written
				off := len(archive) - br.Len()
				end := off + int(hdr.Size)

				if end > len(archive) {
					return io.ErrUnexpectedEOF
				}

				ino.data.setBase(archive[off:end:end])
			}
		case tar.TypeSymlink:
			ino = newInode(S_IFLNK | 0777)
			ino.target = hdr.Linkname
		case tar.TypeLink:
			links = append(links, hardlink{name, cleanPath(hdr.Linkname)})
			continue
		case tar.TypeFifo:
			ino = newInode(S_IFIFO | perm)
		default:
			continue
		}

		ino.uid = uint32(hdr.Uid)
		ino.gid = uint32(hdr.Gid)
		ino.mtime = hdr.ModTime.UnixNano()
		ino.nlink = 0

		if old := dir.children[base]; old != nil {
			dir.unlink(base)
		}

		dir.link(base, ino)
	}

	for _, l := range links {
		target := rootInode

		for c := range strings.SplitSeq(l.target, "/") {
			if c == "" {
				continue
			}

			if target = target.child(c); target == nil {
				break
			}
		}

		if target == nil || target.isDir() {
			continue
		}

		dir := mkdirAll(path.Dir(l.name), 0755)
		dir.link(path.Base(l.name), target)
	}

	return nil
}
