package core

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

type growingSource struct{ remaining, read int }

func (s *growingSource) Read(p []byte) (int, error) {
	if s.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > s.remaining {
		n = s.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 'x'
	}
	s.remaining -= n
	s.read += n
	return n, nil
}
func TestIssue1SourceReadActualLimit(t *testing.T) {
	for _, size := range []int{0, 17, sourceFileLimit, sourceFileLimit + 1, 64 * sourceFileLimit} {
		r := &growingSource{remaining: size}
		b, e := readSourceBytes(r)
		if size > sourceFileLimit {
			if e == nil || e.Error() != "INPUT_TOO_LARGE" || b != nil {
				t.Fatal(size, e)
			}
			if r.read != sourceFileLimit+1 {
				t.Fatal("unbounded read", r.read)
			}
		} else if e != nil || len(b) != size {
			t.Fatal(size, e)
		}
	}
}
func TestIssue1SourceGrowthAfterOpenAndNonRegular(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.json")
	if e := os.WriteFile(path, []byte("[]"), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil || info.Size() != 2 {
		t.Fatal(e)
	}
	// The descriptor was opened/stat'ed at two bytes, then its file grew.
	if e = os.WriteFile(path, bytes.Repeat([]byte("x"), sourceFileLimit+1), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = readSourceBytes(f); e == nil || e.Error() != "INPUT_TOO_LARGE" {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, []byte("[]"), 0600); e != nil {
		t.Fatal(e)
	}
	b, e := readSourceFixture(path)
	if e != nil || string(b) != "[]" {
		t.Fatal(e)
	}
	fifo := filepath.Join(t.TempDir(), "source.fifo")
	if e = syscall.Mkfifo(fifo, 0600); e != nil {
		t.Fatal(e)
	}
	if _, e = readSourceFixture(fifo); e == nil || e.Error() != "FIXTURE_UNAVAILABLE" {
		t.Fatal(e)
	}
	if _, e = readSourceFixture(t.TempDir()); e == nil {
		t.Fatal("directory accepted")
	}
}
