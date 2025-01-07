//go:build !windows && !linux

package gogitstatus

import (
	"os"
	"syscall"
)

func isCTimeUnchanged(stat os.FileInfo, mTimeSec, mTimeNsec int64) bool {
	unixStat := stat.Sys().(*syscall.Stat_t)
	return unixStat.Ctimespec.Sec == mTimeSec && unixStat.Ctimespec.Nsec == mTimeNsec
}
