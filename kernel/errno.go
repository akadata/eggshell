// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package kernel

import (
	"strconv"
)

// Errno represents a Linux error number.
type Errno int

// Linux error numbers (asm-generic/errno-base.h, asm-generic/errno.h)
const (
	EPERM           Errno = 1
	ENOENT          Errno = 2
	ESRCH           Errno = 3
	EINTR           Errno = 4
	EIO             Errno = 5
	ENXIO           Errno = 6
	E2BIG           Errno = 7
	ENOEXEC         Errno = 8
	EBADF           Errno = 9
	ECHILD          Errno = 10
	EAGAIN          Errno = 11
	ENOMEM          Errno = 12
	EACCES          Errno = 13
	EFAULT          Errno = 14
	ENOTBLK         Errno = 15
	EBUSY           Errno = 16
	EEXIST          Errno = 17
	EXDEV           Errno = 18
	ENODEV          Errno = 19
	ENOTDIR         Errno = 20
	EISDIR          Errno = 21
	EINVAL          Errno = 22
	ENFILE          Errno = 23
	EMFILE          Errno = 24
	ENOTTY          Errno = 25
	ETXTBSY         Errno = 26
	EFBIG           Errno = 27
	ENOSPC          Errno = 28
	ESPIPE          Errno = 29
	EROFS           Errno = 30
	EMLINK          Errno = 31
	EPIPE           Errno = 32
	EDOM            Errno = 33
	ERANGE          Errno = 34
	EDEADLK         Errno = 35
	ENAMETOOLONG    Errno = 36
	ENOLCK          Errno = 37
	ENOSYS          Errno = 38
	ENOTEMPTY       Errno = 39
	ELOOP           Errno = 40
	ENOMSG          Errno = 42
	ENODATA         Errno = 61
	ETIME           Errno = 62
	EOVERFLOW       Errno = 75
	EBADFD          Errno = 77
	EILSEQ          Errno = 84
	ENOTSOCK        Errno = 88
	EDESTADDRREQ    Errno = 89
	EMSGSIZE        Errno = 90
	EPROTOTYPE      Errno = 91
	ENOPROTOOPT     Errno = 92
	EPROTONOSUPPORT Errno = 93
	ESOCKTNOSUPPORT Errno = 94
	EOPNOTSUPP      Errno = 95
	EPFNOSUPPORT    Errno = 96
	EAFNOSUPPORT    Errno = 97
	EADDRINUSE      Errno = 98
	EADDRNOTAVAIL   Errno = 99
	ENETDOWN        Errno = 100
	ENETUNREACH     Errno = 101
	ENETRESET       Errno = 102
	ECONNABORTED    Errno = 103
	ECONNRESET      Errno = 104
	ENOBUFS         Errno = 105
	EISCONN         Errno = 106
	ENOTCONN        Errno = 107
	ESHUTDOWN       Errno = 108
	ETIMEDOUT       Errno = 110
	ECONNREFUSED    Errno = 111
	EHOSTDOWN       Errno = 112
	EHOSTUNREACH    Errno = 113
	EALREADY        Errno = 114
	EINPROGRESS     Errno = 115

	// kernel internal: restart system call after signal handling
	ERESTARTSYS Errno = 512
)

func (e Errno) Error() string {
	if s, ok := errnoNames[e]; ok {
		return s
	}
	return "errno " + strconv.Itoa(int(e))
}

var errnoNames = map[Errno]string{
	EPERM:        "EPERM",
	ENOENT:       "ENOENT",
	ESRCH:        "ESRCH",
	EINTR:        "EINTR",
	EIO:          "EIO",
	EBADF:        "EBADF",
	ECHILD:       "ECHILD",
	EAGAIN:       "EAGAIN",
	ENOMEM:       "ENOMEM",
	EACCES:       "EACCES",
	EFAULT:       "EFAULT",
	EEXIST:       "EEXIST",
	ENOTDIR:      "ENOTDIR",
	EISDIR:       "EISDIR",
	EINVAL:       "EINVAL",
	ENOTTY:       "ENOTTY",
	ESPIPE:       "ESPIPE",
	EPIPE:        "EPIPE",
	ENOSYS:       "ENOSYS",
	ENOTEMPTY:    "ENOTEMPTY",
	ELOOP:        "ELOOP",
	ENOTSOCK:     "ENOTSOCK",
	EAFNOSUPPORT: "EAFNOSUPPORT",
	ERESTARTSYS:  "ERESTARTSYS",
	ETIMEDOUT:    "ETIMEDOUT",
	ENOEXEC:      "ENOEXEC",
}
