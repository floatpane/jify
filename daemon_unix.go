//go:build !windows

package main

import "syscall"

// detachSysProcAttr starts the child in a new session so it survives the parent
// shell exiting and is fully detached from the controlling terminal.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
