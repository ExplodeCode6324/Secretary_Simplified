package main

import (
	"bytes"
	"io"
	"os"
	"secretarysimplified/cli/tui"
	"strings"
	"testing"
)

func TestIssue2CLIBooleanFlagsDoNotConsumeCommand(t *testing.T) {
	for _, args := range [][]string{{"--json", "input", "--text", "中文"}, {"chat", "--plain", "--json", "--config", "a.json"}, {"--help", "chat"}} {
		p, f := flags(args)
		if len(p) != 1 {
			t.Fatalf("command swallowed: %q -> %q", args, p)
		}
		if f["json"] != "" && f["json"] != "true" {
			t.Fatal(f)
		}
	}
}
func TestIssue2CLINonTTYDefaultAndHelpDoNotReadInput(t *testing.T) {
	oldArgs, oldIn, oldOut := os.Args, os.Stdin, os.Stdout
	defer func() { os.Args, os.Stdin, os.Stdout = oldArgs, oldIn, oldOut }()
	inR, inW, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer inR.Close()
	defer inW.Close()
	outR, outW, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer outR.Close()
	os.Stdin, os.Stdout = inR, outW
	for _, args := range [][]string{{"secretary"}, {"secretary", "--help"}, {"secretary", "--json"}} {
		os.Args = args
		if e := run(); e != nil {
			t.Fatal(e)
		}
	}
	outW.Close()
	var b bytes.Buffer
	io.Copy(&b, outR)
	if strings.Contains(b.String(), "\x1b") {
		t.Fatal("ANSI in nonTTY")
	}
	if !strings.Contains(b.String(), "secretary") {
		t.Fatal("missing usage")
	}
	if tui.IsTerminal(inR) {
		t.Fatal("pipe mistaken for terminal")
	}
}

func TestIssue2CLIMissingConfigurationDoesNotInitialize(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	dir := t.TempDir()
	os.Args = []string{"secretary", "tui", "--config", dir + "/missing.json"}
	if e := run(); e == nil {
		t.Fatal("missing configuration accepted")
	}
	entries, e := os.ReadDir(dir)
	if e != nil || len(entries) != 0 {
		t.Fatal("client initialized storage", entries, e)
	}
}
