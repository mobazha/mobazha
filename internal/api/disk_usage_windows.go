//go:build windows

package api

import (
	"syscall"
	"unsafe"
)

func getDiskUsage(path string) (totalGB, freeGB, usedPct float64) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDisk := kernel32.NewProc("GetDiskFreeSpaceExW")

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, 0
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	r, _, _ := getDisk.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if r == 0 {
		return 0, 0, 0
	}

	totalGB = float64(totalBytes) / 1024 / 1024 / 1024
	freeGB = float64(freeBytesAvailable) / 1024 / 1024 / 1024
	if totalGB > 0 {
		usedPct = (1 - freeGB/totalGB) * 100
	}
	return
}
