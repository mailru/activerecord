package time

import "fmt"

/*
#cgo LDFLAGS: -lrt
#include <time.h>
*/
import "C"

func Monotonic() (ret MonotonicTimestamp, err error) {
	var ts C.struct_timespec

	code := C.clock_gettime(C.CLOCK_MONOTONIC, &ts)
	if code != 0 {
		err = fmt.Errorf("clock_gettime error: %d", code)
		return
	}

	ret.sec = int64(ts.tv_sec)
	ret.nsec = int32(ts.tv_nsec)

	return
}
