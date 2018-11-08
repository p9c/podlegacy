// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package server

import (
	"os"
	"syscall"
)

func init() {
	InterruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
}
