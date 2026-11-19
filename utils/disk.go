package utils

import (
	"os"
	"syscall"
)

// FreeBytes reports the space available to unprivileged processes on the
// filesystem that holds path. It stats the VOLUME, not the directory: what
// matters is whether the disk backing the uploads can still accept a write.
//
// On the VPS layout this matters more than it looks — Postgres and the uploads
// typically share one volume, so filling `/uploads` takes the database down with
// it. That is why the guard is about free space rather than a per-file cap: a
// full volume is an outage, a rejected upload is a 503.
func FreeBytes(path string) (uint64, error) {
	if path == "" {
		path = "."
	}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	return stat.Bavail * uint64(stat.Bsize), nil
}

// UploadsRoot resolves the directory whose volume must have room before an
// upload is accepted. UPLOAD_DIR (receipts) wins when configured, then the
// conventional ./uploads, then the working directory.
func UploadsRoot() string {
	if dir := os.Getenv("UPLOAD_DIR"); dir != "" {
		return dir
	}
	if info, err := os.Stat("uploads"); err == nil && info.IsDir() {
		return "uploads"
	}
	return "."
}
