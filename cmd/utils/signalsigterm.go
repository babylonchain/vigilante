// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package utils

import (
	"os"
	"syscall"
)

func init() {
	signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
}
