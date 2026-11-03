// Copyright (c) The eggshell Authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// Package microvm provides hardware initialization, automatically on import,
// for a QEMU microvm machine bridging its tamago library package.
//
// This package is only meant to be used with `GOOS=tamago GOARCH=amd64` as
// supported by the TamaGo framework for bare metal Go, see
// https://github.com/usbarmory/tamago.
package microvm

import (
	"github.com/usbarmory/tamago/board/qemu/microvm"
	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/kvm/clock"
	"github.com/usbarmory/tamago/kvm/virtio"

	"github.com/usbarmory/go-net"
	"github.com/usbarmory/go-net/virtio"
)

const (
	// VirtIO Memory-mapped I/O
	VIRTIO_MMIO_BASE = 0xfeb00000

	// VirtIO Networking (first -device virtio-net-device)
	VIRTIO_NET0_BASE = VIRTIO_MMIO_BASE + 0x2e00
	VIRTIO_NET0_IRQ  = 47
)

var (
	CPU     = microvm.AMD64
	IOAPIC0 = microvm.IOAPIC0
	IOAPIC1 = microvm.IOAPIC1
	UART0   = microvm.UART0

	VIRTIO_NET0 = &vnet.Net{
		Transport: &virtio.MMIO{
			Base: VIRTIO_NET0_BASE,
		},
		IRQ:          VIRTIO_NET0_IRQ,
		HeaderLength: 10,
		MTU:          gnet.MTU,
	}

	VIRTIO_NET0_IOAPIC = IOAPIC1
)

func init() {
	if CPU.Features().KVM {
		microvm.AMD64.SetTime(kvmclock.Now().UnixNano())
	} else {
		if t, err := microvm.RTC.Now(); err == nil {
			microvm.AMD64.SetTime(t.UnixNano())
		}
	}

	// override global DMA region
	dma.Init(DMAStart, DMASize)
}
