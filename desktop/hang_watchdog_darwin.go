//go:build darwin

package main

/*
#include <stdint.h>
#include <dispatch/dispatch.h>

extern void vcodeDesktopMainHeartbeat(void);

static dispatch_source_t vcode_main_heartbeat_timer;

static void vcode_main_heartbeat_handler(void *ctx) {
	vcodeDesktopMainHeartbeat();
}

static void vcode_start_main_heartbeat(uint64_t interval_ms) {
	if (vcode_main_heartbeat_timer != NULL) {
		return;
	}
	vcode_main_heartbeat_timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, dispatch_get_main_queue());
	dispatch_set_context(vcode_main_heartbeat_timer, NULL);
	dispatch_source_set_event_handler_f(vcode_main_heartbeat_timer, vcode_main_heartbeat_handler);
	dispatch_source_set_timer(vcode_main_heartbeat_timer, dispatch_time(DISPATCH_TIME_NOW, 0), interval_ms * NSEC_PER_MSEC, 100 * NSEC_PER_MSEC);
	dispatch_resume(vcode_main_heartbeat_timer);
}

static void vcode_stop_main_heartbeat(void) {
	if (vcode_main_heartbeat_timer == NULL) {
		return;
	}
	dispatch_source_cancel(vcode_main_heartbeat_timer);
	vcode_main_heartbeat_timer = NULL;
}
*/
import "C"

import "time"

func mainThreadWatchdogSupported() bool {
	return true
}

func startNativeMainThreadHeartbeat(intervalMS uint64) {
	C.vcode_start_main_heartbeat(C.uint64_t(intervalMS))
}

func stopNativeMainThreadHeartbeat() {
	C.vcode_stop_main_heartbeat()
}

//export vcodeDesktopMainHeartbeat
func vcodeDesktopMainHeartbeat() {
	recordMainThreadHeartbeat(time.Now())
}
