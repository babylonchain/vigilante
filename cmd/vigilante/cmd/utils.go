// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2013-2014 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build darwin dragonfly freebsd linux netbsd openbsd solaris

package cmd

import (
	vlog "github.com/babylonchain/vigilante/log"
	"os"
	"os/signal"
	"syscall"
)

// Copyright (c) 2022-2022 The Babylon developers
// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

func init() {
	signals = []os.Signal{os.Interrupt, syscall.SIGTERM}
}

// interruptChannel is used to receive SIGINT (Ctrl+C) signals.
var interruptChannel chan os.Signal

// addHandlerChannel is used to add an interrupt handler to the list of handlers
// to be invoked on SIGINT (Ctrl+C) signals.
var addHandlerChannel = make(chan func())

// InterruptHandlersDone is closed after all interrupt handlers run the first
// time an interrupt is signaled.
var interruptHandlersDone = make(chan struct{})

var simulateInterruptChannel = make(chan struct{}, 1)

// signals defines the signals that are handled to do a clean shutdown.
// Conditional compilation is used to also include SIGTERM on Unix.
var signals = []os.Signal{os.Interrupt}

// mainInterruptHandler listens for SIGINT (Ctrl+C) signals on the
// interruptChannel and invokes the registered interruptCallbacks accordingly.
// It also listens for callback registration.  It must be run as a goroutine.
func mainInterruptHandler() {
	// interruptCallbacks is a list of callbacks to invoke when a
	// SIGINT (Ctrl+C) is received.
	var interruptCallbacks []func()
	invokeCallbacks := func() {
		// run handlers in LIFO order.
		for i := range interruptCallbacks {
			idx := len(interruptCallbacks) - 1 - i
			interruptCallbacks[idx]()
		}
		close(interruptHandlersDone)
	}

	for {
		select {
		case sig := <-interruptChannel:
			vlog.Logger.WithField("module", "cmd").Infof("Received signal (%s).  Shutting down...", sig)
			invokeCallbacks()
			return
		case <-simulateInterruptChannel:
			vlog.Logger.WithField("module", "cmd").Infof("Received shutdown request.  Shutting down...")
			invokeCallbacks()
			return

		case handler := <-addHandlerChannel:
			interruptCallbacks = append(interruptCallbacks, handler)
		}
	}
}

// AddInterruptHandler adds a handler to call when a SIGINT (Ctrl+C) is
// received.
func addInterruptHandler(handler func()) {
	// Create the channel and start the main interrupt handler which invokes
	// all other callbacks and exits if not already done.
	if interruptChannel == nil {
		interruptChannel = make(chan os.Signal, 1)
		signal.Notify(interruptChannel, signals...)
		go mainInterruptHandler()
	}

	addHandlerChannel <- handler
}
