// emptydb creates a credential-free SQLite schema artifact for release inspection.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/store"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "emptydb output-directory")
		os.Exit(2)
	}
	dir, _ := filepath.Abs(os.Args[1])
	s, e := store.Init(filepath.Join(dir, "secretary.sqlite"), filepath.Join(dir, "objects"))
	if e == nil {
		_, e = s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		s.Close()
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
