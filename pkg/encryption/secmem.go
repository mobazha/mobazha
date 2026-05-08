package encryption

// ZeroBytes overwrites the slice with zeros to scrub key material from
// memory before the underlying buffer is freed or returned to a pool.
//
// Go has no guarantee a zeroed buffer won't be copied to disk by the runtime
// or paged out before zeroing — this is best-effort. Pair with `defer
// ZeroBytes(...)` immediately after deriving any private key bytes.
func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
