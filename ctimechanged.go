//go:build !windows
// +build !windows

package gogitstatus

import (
	"os"
	"syscall"
)

func isCTimeUnchanged(stat os.FileInfo, mTimeSec, mTimeNsec int64) bool {
	unixStat := stat.Sys().(*syscall.Stat_t)
	return unixStat.Ctim.Sec == mTimeSec && unixStat.Ctim.Nano() == mTimeNsec
}
