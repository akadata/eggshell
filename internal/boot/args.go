// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package boot

import (
	"strconv"
	"strings"
)

type Options struct {
	Init     []string
	Hostname string
	Address  string
	Gateway  string
	SSH      uint64
	Trace    bool
	Quiet    bool
	Env      []string
}

func splitArgs(s string) (args []string) {
	var cur strings.Builder
	var quote rune
	inArg := false
	escape := false

	for _, c := range s {
		switch {
		case escape:
			cur.WriteRune(c)
			escape = false
		case c == '\\' && quote != '\'':
			escape = true
			inArg = true
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				cur.WriteRune(c)
			}
		case c == '"' || c == '\'':
			quote = c
			inArg = true
		case c == ' ' || c == '\t' || c == '\n':
			if inArg {
				args = append(args, cur.String())
				cur.Reset()
				inArg = false
			}
		default:
			cur.WriteRune(c)
			inArg = true
		}
	}

	if inArg {
		args = append(args, cur.String())
	}

	return
}

// Parse parses `key=value ... [-- command args...]`.
func (o *Options) Parse(s string) {
	var fields []string

	for _, f := range splitArgs(s) {
		// device parameters appended by QEMU (microvm without ACPI)
		if strings.HasPrefix(f, "virtio_mmio.") {
			continue
		}

		fields = append(fields, f)
	}

	for i, f := range fields {
		if f == "--" {
			o.Init = fields[i+1:]
			break
		}

		k, v, _ := strings.Cut(f, "=")

		switch k {
		case "init":
			o.Init = []string{v}
		case "strace":
			o.Trace = true
		case "quiet":
			o.Quiet = true
		case "ip":
			o.Address = v
		case "gw":
			o.Gateway = v
		case "hostname":
			o.Hostname = v
		case "env":
			o.Env = append(o.Env, v)
		case "ssh":
			p, _ := strconv.Atoi(v)
			o.SSH = uint64(p)
		}
	}
}
