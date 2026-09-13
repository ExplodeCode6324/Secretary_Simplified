// modelprobe uses the same strict Context builder and provider as deployed Core.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"secretarysimplified/config"
	ctxbuild "secretarysimplified/context"
	"secretarysimplified/contract"
	"secretarysimplified/model"
	"secretarysimplified/store"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("config path required")
	}
	c, e := config.Load(os.Args[1])
	if e != nil {
		return e
	}
	dir, e := os.MkdirTemp("", "secretary-model-probe-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(dir)
	s, e := store.Init(filepath.Join(dir, "state.sqlite"), filepath.Join(dir, "objects"))
	if e != nil {
		return e
	}
	defer s.Close()
	id := contract.NewID()
	in := contract.InputEnvelope{SchemaVersion: 1, RequestID: id, SessionID: id, PrincipalID: "master", Origin: "MASTER_CLI", ReceivedAt: contract.Now(), Text: "This is synthetic test data. Reply text exactly PROBE_OK. No actions, controls, or evidence.", AttachmentRefs: []contract.ObjectRef{}, DataClass: "SYNTHETIC", Extensions: map[string]any{}}
	t, e := s.AcceptInput(context.Background(), in, 100)
	if e != nil {
		return e
	}
	p := model.New(c)
	b := ctxbuild.Builder{Store: s, Model: p, Config: c}
	req, _, e := b.Build(context.Background(), t, time.Now())
	if e != nil {
		return e
	}
	r, e := p.Generate(context.Background(), req)
	if e != nil {
		return e
	}
	raw, _ := json.MarshalIndent(map[string]any{"profile": c.Model.Profile, "model": c.Model.Model, "endpoint": c.Model.Endpoint, "request_bytes": r.RequestBytes, "input_tokens": r.InputTokens, "output_tokens": r.OutputTokens, "duration_ms": r.Duration.Milliseconds(), "output": json.RawMessage(r.Output)}, "", "  ")
	fmt.Println(string(raw))
	return nil
}
