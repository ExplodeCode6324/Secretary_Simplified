package tests

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"secretarysimplified/contract"
	"secretarysimplified/store"
	"secretarysimplified/transport"
	"strings"
	"testing"
	"time"
)

func TestAcceptanceA21ReleaseRunnerOfflineAlarmControls(t *testing.T) {
	binary, e := filepath.Abs("../../release/secretaryd")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(binary); e != nil {
		t.Skip("release binary unavailable")
	}
	dir, e := os.MkdirTemp("/tmp", "ss-alarm-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(dir)
	cli := filepath.Join(filepath.Dir(binary), "secretary")
	if out, e := exec.Command(cli, "init", "--data-dir", dir).CombinedOutput(); e != nil {
		t.Fatalf("init %v %s", e, out)
	}
	s, e := store.Open(filepath.Join(dir, "state", "secretary.sqlite"), filepath.Join(dir, "objects"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	ctx := context.Background()
	g := probeGrant(t, s, []string{"alarm.play"}, []string{})
	audio, e := s.PutObject(ctx, []byte("synthetic muted fixture; never decoded"), "audio/wav", "SYNTHETIC")
	if e != nil {
		t.Fatal(e)
	}
	cmd := contract.Command{SchemaVersion: 1, OperationKey: "alarm", Capability: "alarm.play", CapabilityVersion: 1, Arguments: map[string]any{"audio_ref": audio, "device_id": "synthetic-muted", "max_duration_seconds": 120}, ExpectedRevisions: []contract.ReadRef{}, Extensions: map[string]any{"security.classification": map[string]any{"data_class": "SYNTHETIC"}}}
	criteria := []contract.Criterion{{ID: contract.NewID(), Kind: "notification_recorded", Expected: map[string]any{"notification_key": "alarm-test-only"}, EvidencePolicy: "Fixture: verifier completion is outside this controller test"}}
	if _, e = s.RegisterImmediate(ctx, contract.NewID(), contract.NewID(), cmd, criteria, g); e != nil {
		t.Fatal(e)
	}
	log, e := os.Create(filepath.Join(dir, "runner.log"))
	if e != nil {
		t.Fatal(e)
	}
	defer log.Close()
	p := exec.Command(binary, "runner", "--config", filepath.Join(dir, "config.json"))
	p.Stdout = log
	p.Stderr = log
	if e = p.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { p.Process.Signal(os.Interrupt); p.Wait() }()
	token, e := os.ReadFile(filepath.Join(dir, "run", "client.token"))
	if e != nil {
		t.Fatal(e)
	}
	client := transport.Client{Socket: filepath.Join(dir, "run", "runner.sock"), Token: strings.TrimSpace(string(token))}
	waitAlarm := func(exclude string) string {
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			var raw string
			e := s.DB.QueryRow(`SELECT payload_json FROM alarm_session WHERE state='PLAYING' AND id<>? LIMIT 1`, exclude).Scan(&raw)
			if e == nil {
				var a contract.AlarmSession
				if e = contract.Decode("AlarmSession", []byte(raw), &a); e != nil {
					t.Fatal(e)
				}
				var m map[string]any
				json.Unmarshal([]byte(raw), &m)
				settings := m["saved_settings"].(map[string]any)
				if settings["muted"] != true || settings["output_volume"] != float64(0) || !strings.HasPrefix(m["playback_handle"].(string), "muted:") {
					t.Fatal("not muted", settings)
				}
				return a.ID
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("alarm missing")
		return ""
	}
	first := waitAlarm("")
	if _, e = os.Stat(filepath.Join(dir, "run", "core-internal.sock")); !os.IsNotExist(e) {
		t.Fatal("Core unexpectedly online")
	}
	request := contract.NewID()
	body := map[string]any{"schema_version": 1, "request_id": request, "delay_seconds": 1}
	for i := 0; i < 2; i++ {
		_, status, e := client.Call(ctx, "POST", "/v1/alarms/"+first+"/snooze", body)
		if e != nil || status != 200 {
			t.Fatal("offline snooze", status, e)
		}
	}
	second := waitAlarm(first)
	var n int
	s.DB.QueryRow(`SELECT count(*) FROM scheduled_job`).Scan(&n)
	if n != 1 {
		t.Fatal("snooze retry duplicate", n)
	}
	_, status, e := client.Call(ctx, "POST", "/v1/alarms/"+second+"/stop", map[string]any{"schema_version": 1, "request_id": contract.NewID()})
	if e != nil || status != 200 {
		t.Fatal("offline stop", status, e)
	}
	var state string
	s.DB.QueryRow(`SELECT state FROM alarm_session WHERE id=?`, second).Scan(&state)
	if state != "STOPPED" {
		t.Fatal(state)
	}
	_, status, e = client.Call(ctx, "GET", "/v1/health", nil)
	if e != nil || status != 200 {
		t.Fatal("offline control health", status, e)
	}
}
