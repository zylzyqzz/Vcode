//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void installVcodeSystemQuitHook(void);
*/
import "C"

import "sync"

var installSystemQuitHookOnce sync.Once

func installSystemQuitHook() {
	installSystemQuitHookOnce.Do(func() {
		C.installVcodeSystemQuitHook()
	})
}

//export VcodeMarkSystemQuit
func VcodeMarkSystemQuit() {
	markSystemQuitRequested()
}
