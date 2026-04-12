//go:build !windows

package api

import "golang.org/x/sys/unix"

func getDiskUsage(path string) (totalGB, freeGB, usedPct float64) {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return 0, 0, 0
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)

	totalGB = float64(total) / 1024 / 1024 / 1024
	freeGB = float64(free) / 1024 / 1024 / 1024
	if totalGB > 0 {
		usedPct = (1 - freeGB/totalGB) * 100
	}
	return
}
