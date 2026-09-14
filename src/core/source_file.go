package core

import (
	"errors"
	"io"
	"os"
	"syscall"
)

const sourceFileLimit = 1 << 20

// readSourceFixture bounds the actual read, including files that grow after open.
// Nonblocking open prevents a misconfigured FIFO from blocking before Stat.
func readSourceFixture(path string) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if e != nil {
		return nil, errors.New("FIXTURE_UNAVAILABLE")
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || !info.Mode().IsRegular() {
		return nil, errors.New("FIXTURE_UNAVAILABLE")
	}
	return readSourceBytes(f)
}
func readSourceBytes(reader io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(reader, sourceFileLimit+1))
	if e != nil {
		return nil, errors.New("FIXTURE_UNAVAILABLE")
	}
	if len(b) > sourceFileLimit {
		return nil, errors.New("INPUT_TOO_LARGE")
	}
	return b, nil
}
