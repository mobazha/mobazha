// +build !darwin
// +build !linux
// +build !netbsd
// +build !openbsd

package repo

// CheckAndSetUlimit is a no-op on non-unix systems
func CheckAndSetUlimit() error {
	return nil
}
