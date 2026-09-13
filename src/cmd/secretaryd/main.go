// secretaryd hosts Core or the independently controlled Runner on private Unix sockets.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"secretarysimplified/config"
	"secretarysimplified/core"
	"secretarysimplified/diagnostics"
	"secretarysimplified/executor"
	"secretarysimplified/model"
	"secretarysimplified/platform"
	"secretarysimplified/store"
	"secretarysimplified/transport"
	"strings"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil && !errors.Is(e, context.Canceled) {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: secretaryd core|runner --config PATH")
	}
	role := os.Args[1]
	if role != "core" && role != "runner" {
		return errors.New("unknown daemon role")
	}
	fs := flag.NewFlagSet(role, flag.ContinueOnError)
	configPath := fs.String("config", "release/config.json", "configuration file")
	if e := fs.Parse(os.Args[2:]); e != nil {
		return e
	}
	c, e := config.Load(*configPath)
	if e != nil {
		return e
	}
	runDir := filepath.Join(c.DataDir, "run")
	for _, socket := range []string{"core.sock", "core-internal.sock", "runner.sock"} {
		if len([]byte(filepath.Join(runDir, socket))) >= 104 {
			return errors.New("UNIX_SOCKET_PATH_TOO_LONG: choose a shorter data_dir (socket path must be below 104 bytes)")
		}
	}
	if e = os.MkdirAll(runDir, 0700); e != nil {
		return e
	}
	lock, e := platform.AcquireLock(filepath.Join(runDir, role+".lock"))
	if e != nil {
		return e
	}
	defer lock.Close()
	readToken := func(name string) (string, error) {
		b, e := os.ReadFile(filepath.Join(runDir, name))
		if e != nil {
			return "", e
		}
		token := strings.TrimSpace(string(b))
		if len(token) < 32 {
			return "", errors.New("invalid local authentication token")
		}
		return token, nil
	}
	clientToken, e := readToken("client.token")
	if e != nil {
		return e
	}
	internalToken, e := readToken("internal.token")
	if e != nil {
		return e
	}
	if clientToken == internalToken {
		return errors.New("client and internal tokens must differ")
	}
	s, e := store.Open(filepath.Join(c.DataDir, "state", "secretary.sqlite"), filepath.Join(c.DataDir, "objects"))
	if e != nil {
		return e
	}
	defer s.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	frozen := func() bool {
		if c.Frozen {
			return true
		}
		_, e := os.Stat(filepath.Join(c.DataDir, "execution_frozen"))
		return e == nil || !os.IsNotExist(e)
	}
	errs := make(chan error, 5)
	heartbeat := func(scan bool) {
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		payload := map[string]any{"at": now}
		if scan {
			payload["last_scan_at"] = now
		}
		b, _ := json.Marshal(payload)
		tmp := filepath.Join(runDir, role+".heartbeat.tmp")
		if os.WriteFile(tmp, b, 0600) == nil {
			_ = os.Rename(tmp, filepath.Join(runDir, role+".heartbeat.json"))
		}
	}
	report := func(component string, e error) {
		if e != nil && ctx.Err() == nil {
			fmt.Fprintln(os.Stderr, component+": operation failed; inspect persisted diagnostics")
		}
	}

	if role == "core" {
		b, e := os.ReadFile(filepath.Join(runDir, "grant.id"))
		if e != nil {
			return e
		}
		provider := &diagnostics.RecordingModel{Inner: model.New(c), Store: s, Config: c, Dir: filepath.Join(c.DataDir, "reports", "model_calls")}
		service := &core.Service{Store: s, Model: provider, Config: c, GrantID: strings.TrimSpace(string(b))}
		go func() {
			errs <- transport.Serve(ctx, filepath.Join(runDir, "core.sock"), clientToken, service.Handler())
		}()
		go func() {
			errs <- transport.Serve(ctx, filepath.Join(runDir, "core-internal.sock"), internalToken, service.InternalHandler())
		}()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				if !frozen() {
					e := service.Step(ctx)
					report("core", e)
					heartbeat(e == nil)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	} else {
		grantBytes, readErr := os.ReadFile(filepath.Join(runDir, "grant.id"))
		if readErr != nil {
			return readErr
		}
		grantID := strings.TrimSpace(string(grantBytes))
		epoch, _ := time.Parse(time.RFC3339Nano, c.Epoch)
		r := executor.New(s, &executor.Options{ExecutorID: "local-runner", ArtifactDir: filepath.Join(c.DataDir, "objects", "artifacts"), Frozen: frozen, Core: executor.RemoteCore{Client: transport.Client{Socket: filepath.Join(runDir, "core-internal.sock"), Token: internalToken}}})
		go func() { errs <- transport.Serve(ctx, filepath.Join(runDir, "runner.sock"), clientToken, r.Handler()) }()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				if !frozen() {
					_, slotErr := s.ScheduleMemorySlot(ctx, epoch, time.Now(), grantID)
					report("memory slot controller", slotErr)
					alarmErr := s.ExpireMutedAlarms(ctx, time.Now())
					report("alarm duration", alarmErr)
					_, wakeErr := s.WakeWaits(ctx, time.Now())
					report("wait scanner", wakeErr)
					_, scanErr := s.ScheduleStep(ctx, time.Now())
					report("scheduler", scanErr)
					heartbeat(scanErr == nil)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				if !frozen() {
					_, stepErr := r.Step(ctx)
					report("executor", stepErr)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()

	}
	select {
	case <-ctx.Done():
		return nil
	case e := <-errs:
		cancel()
		return e
	}
}
