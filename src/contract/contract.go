package contract

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"strings"
	"sync"
	"time"
)

//go:embed contracts.schema.json
var SchemaJSON []byte
var once sync.Once
var schemas map[string]*jsonschema.Schema
var schemaErr error

func initSchemas() {
	var doc map[string]any
	if schemaErr = json.Unmarshal(SchemaJSON, &doc); schemaErr != nil {
		return
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	schemaErr = c.AddResource("urn:secretary-simplified:contracts:v1", doc)
	if schemaErr != nil {
		return
	}
	schemas = map[string]*jsonschema.Schema{}
	for n := range doc["$defs"].(map[string]any) {
		var s *jsonschema.Schema
		s, schemaErr = c.Compile("urn:secretary-simplified:contracts:v1#/$defs/" + n)
		if schemaErr != nil {
			return
		}
		schemas[n] = s
	}
}
func Validate(name string, v any) error {
	once.Do(initSchemas)
	if schemaErr != nil {
		return schemaErr
	}
	s, ok := schemas[name]
	if !ok {
		return fmt.Errorf("UNKNOWN_TYPE: %s", name)
	}
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	var data any
	if e = json.Unmarshal(b, &data); e != nil {
		return e
	}
	if e = s.Validate(data); e != nil {
		return fmt.Errorf("INVALID_CONTRACT %s: %w", name, e)
	}
	if name == "Context" {
		m := data.(map[string]any)
		seen := map[string]bool{}
		for _, raw := range m["sections"].([]any) {
			n := raw.(map[string]any)["name"].(string)
			if seen[n] {
				return fmt.Errorf("DUPLICATE_CONTEXT_SECTION")
			}
			seen[n] = true
		}
	}
	return validateExtensions(data)
}
func Decode(name string, b []byte, out any) error {
	v, e := ParseJSON(b)
	if e != nil {
		return e
	}
	if e = Validate(name, v); e != nil {
		return e
	}
	return json.Unmarshal(b, out)
}
func Schema(name string) json.RawMessage {
	var d map[string]any
	json.Unmarshal(SchemaJSON, &d)
	all := d["$defs"].(map[string]any)
	defs := map[string]any{}
	var walk func(any)
	var add func(string)
	add = func(n string) {
		if _, ok := defs[n]; ok {
			return
		}
		v, ok := all[n]
		if !ok {
			return
		}
		defs[n] = v
		walk(v)
	}
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			delete(x, "description")
			delete(x, "title")
			if r, ok := x["$ref"].(string); ok && strings.HasPrefix(r, "#/$defs/") {
				add(strings.TrimPrefix(r, "#/$defs/"))
			}
			for _, w := range x {
				walk(w)
			}
		case []any:
			for _, w := range x {
				walk(w)
			}
		}
	}
	add(name)
	b, _ := json.Marshal(map[string]any{"$schema": d["$schema"], "$ref": "#/$defs/" + name, "$defs": defs})
	return b
}
func NewID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func Now() string                  { return Timestamp(time.Now()) }
func Timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z") }
func NormalizeTimestamp(s string) (string, error) {
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		return "", e
	}
	return Timestamp(t), nil
}
func Hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func ValidateRegistered(kind string, v any) error {
	n, ok := map[string]string{"fixture.item": "FixtureItemValue", "device.availability": "DeviceAvailabilityValue", "source.health": "SourceHealthValue", "master.preference": "MasterPreferenceValue", "project.background": "ProjectBackgroundValue", "entity.relation": "EntityRelationValue"}[kind]
	if !ok {
		return fmt.Errorf("UNKNOWN_TYPE: %s", kind)
	}
	return Validate(n, v)
}
func ValidateEvent(e ChangeEvent) error {
	if err := Validate("ChangeEvent", e); err != nil {
		return err
	}
	typ, ok := map[string]string{"item.created": "Item", "item.updated": "Item", "task.updated": "Task", "world.updated": "WorldFact", "run.updated": "JobRun", "source.synced": "SourceState", "memory.refreshed": "ConsciousnessState", "scheduled_job.updated": "ScheduledJob", "scheduled_job.skipped": "ScheduledJob"}[e.EventType]
	if !ok {
		return fmt.Errorf("UNKNOWN_EVENT: %s", e.EventType)
	}
	if e.Change["before"] == nil && e.Change["after"] == nil {
		return fmt.Errorf("INVALID_EVENT: empty change")
	}
	for k, v := range e.Change {
		if k != "before" && k != "after" && k != "evidence" {
			return fmt.Errorf("INVALID_EVENT: unknown change field")
		}
		if (k == "before" || k == "after") && v != nil {
			if err := Validate(typ, v); err != nil {
				return err
			}
			if k == "after" {
				b, _ := json.Marshal(v)
				var m map[string]any
				json.Unmarshal(b, &m)
				if r, ok := m["revision"].(float64); ok && int(r) != e.EntityRevision {
					return fmt.Errorf("INVALID_EVENT: revision mismatch")
				}
			}
		}
	}
	if !strings.EqualFold(e.EntityType, typ) && !(e.EntityType == "source" && typ == "SourceState") && !(e.EntityType == "world_fact" && typ == "WorldFact") && !(e.EntityType == "run" && typ == "JobRun") && !(e.EntityType == "memory" && typ == "ConsciousnessState") && !(e.EntityType == "scheduled_job" && typ == "ScheduledJob") {
		return fmt.Errorf("INVALID_EVENT: entity type")
	}
	return nil
}

// Extensions are inert unless their namespace and shape are registered here.
func validateExtensions(v any) error {
	switch x := v.(type) {
	case []any:
		for _, v := range x {
			if e := validateExtensions(v); e != nil {
				return e
			}
		}
	case map[string]any:
		if ex, ok := x["extensions"].(map[string]any); ok {
			for k, raw := range ex {
				m, ok := raw.(map[string]any)
				if !ok {
					return fmt.Errorf("INVALID_EXTENSION: %s", k)
				}
				switch k {
				case "runtime.authorization":
					if len(m) != 1 {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					id, ok := m["grant_id"].(string)
					if !ok {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					if e := schemas["VersionRef"].Validate(map[string]any{"id": id, "revision": float64(1)}); e != nil {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
				case "runtime.artifacts":
					refs, ok := m["refs"].([]any)
					if len(m) != 1 || !ok || len(refs) > 20 {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					for _, ref := range refs {
						if e := schemas["ObjectRef"].Validate(ref); e != nil {
							return fmt.Errorf("INVALID_EXTENSION: %s", k)
						}
					}
				case "context.retrieval":
					events, ok := m["events"].([]any)
					n, nok := m["omitted_count"].(float64)
					_, cursorOK := m["next_cursor"].(string)
					if len(m) != 3 || !ok || len(events) > 10 || !nok || n < 0 || n != float64(int(n)) || (!cursorOK && m["next_cursor"] != nil) {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					for _, event := range events {
						if e := schemas["ConversationEvent"].Validate(event); e != nil {
							return fmt.Errorf("INVALID_EXTENSION: %s", k)
						}
					}
				case "runtime.calendar_skip":
					date, dok := m["local_date"].(string)
					zone, zok := m["timezone"].(string)
					local, lok := m["local_time"].(string)
					reason, rok := m["reason"].(string)
					if len(m) != 4 || !dok || !zok || !lok || !rok || reason != "DST_GAP" {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					if _, e := time.Parse("2006-01-02", date); e != nil {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					if _, e := time.LoadLocation(zone); e != nil {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
					if _, e := time.Parse("15:04", local); e != nil {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
				case "runtime.cancellation":
					n, ok := m["cancel_generation"].(float64)
					_, boolean := m["effect_observed"].(bool)
					if len(m) != 2 || !ok || !boolean || n < 1 || n != float64(int(n)) {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
				case "runtime.misfire":
					n, ok := m["omitted_occurrences"].(float64)
					if len(m) != 1 || !ok || n < 0 || n != float64(int(n)) {
						return fmt.Errorf("INVALID_EXTENSION: %s", k)
					}
				default:
					return fmt.Errorf("UNREGISTERED_EXTENSION: %s", k)
				}
			}
		}
		for k, v := range x {
			if k != "extensions" && k != "output_contract" {
				if e := validateExtensions(v); e != nil {
					return e
				}
			}
		}
	}
	return nil
}

// DeriveID creates a stable UUID-shaped identifier from a versioned namespace key.
func DeriveID(key string) string {
	b := sha256.Sum256([]byte(key))
	b[6] = (b[6] & 15) | 80
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// ValidationPaths reports only instance locations, never offending values.
func ValidationPaths(err error) []string {
	var validation *jsonschema.ValidationError
	if !errors.As(err, &validation) {
		return []string{}
	}
	out := []string{}
	seen := map[string]bool{}
	var walk func(*jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			path := "/" + strings.Join(e.InstanceLocation, "/")
			if !seen[path] && len(out) < 10 {
				out = append(out, path)
				seen[path] = true
			}
		}
		for _, child := range e.Causes {
			walk(child)
		}
	}
	walk(validation)
	return out
}
