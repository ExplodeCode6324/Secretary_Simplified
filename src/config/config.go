// Package config owns explicit, versioned local deployment configuration.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Model struct {
	Profile        string `json:"profile"`
	Endpoint       string `json:"endpoint"`
	Model          string `json:"model"`
	SecretRef      string `json:"secret_ref"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}
type ProviderPolicy struct {
	AllowedClasses []string `json:"allowed_data_classes"`
	SourceIDs      []string `json:"source_ids"`
}
type Limits struct {
	Queue        int `json:"queue"`
	RequestBytes int `json:"request_bytes"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}
type SourceConfig struct {
	ID          string `json:"id"`
	FixturePath string `json:"fixture_path"`
}
type Config struct {
	SchemaVersion int            `json:"schema_version"`
	DataDir       string         `json:"data_dir"`
	Timezone      string         `json:"timezone"`
	Model         Model          `json:"model_profile"`
	Policy        ProviderPolicy `json:"provider_policy"`
	Limits        Limits         `json:"limits"`
	Epoch         string         `json:"consciousness_epoch_at"`
	Frozen        bool           `json:"execution_frozen"`
	TestEntityIDs []string       `json:"test_entity_ids"`
	SourceConfigs []SourceConfig `json:"source_configs"`
}

func Default(dir string) Config {
	return Config{1, dir, "Asia/Hong_Kong", Model{"fixture", "https://opencode.ai/zen/go/v1/responses", "gpt-5.6-luna", "", 60}, ProviderPolicy{[]string{"SYNTHETIC"}, []string{}}, Limits{100, 65536, 32000, 2000}, time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), false, []string{"00000000-0000-4000-8000-000000000100"}, []SourceConfig{}}
}
func Load(path string) (Config, error) {
	var c Config
	f, e := os.Open(path)
	if e != nil {
		return c, e
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(&c); e != nil {
		return c, e
	}
	if !filepath.IsAbs(c.DataDir) {
		c.DataDir = filepath.Join(filepath.Dir(path), c.DataDir)
	}
	if c.Model.SecretRef != "" && !filepath.IsAbs(c.Model.SecretRef) {
		c.Model.SecretRef = filepath.Join(filepath.Dir(path), c.Model.SecretRef)
	}
	return c, c.Validate()
}
func (c Config) Validate() error {
	if c.SchemaVersion != 1 {
		return errors.New("UNSUPPORTED_VERSION")
	}
	if _, e := time.LoadLocation(c.Timezone); e != nil {
		return e
	}
	if _, e := time.Parse(time.RFC3339Nano, c.Epoch); e != nil {
		return e
	}
	if c.Model.Profile != "fixture" && c.Model.Profile != "opencode-go" {
		return errors.New("unsupported model profile")
	}
	if c.Model.Profile == "opencode-go" && (c.Model.Endpoint != "https://opencode.ai/zen/go/v1/responses" || c.Model.Model != "gpt-5.6-luna") {
		return errors.New("unexpected provider endpoint or model")
	}
	if c.Limits.Queue < 1 || c.Limits.Queue > 100 || c.Limits.RequestBytes < 1024 || c.Limits.InputTokens < 1024 || c.Limits.OutputTokens < 1 {
		return errors.New("invalid limits")
	}
	for _, v := range c.Policy.AllowedClasses {
		if v == "SECRET" {
			return errors.New("SECRET disclosure forbidden")
		}
	}
	return nil
}
func (c Config) Save(path string) error {
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(path, append(b, '\n'), 0600)
}
func (c Config) Allows(class string) bool {
	if class == "SECRET" {
		return false
	}
	for _, v := range c.Policy.AllowedClasses {
		if class == v {
			return true
		}
	}
	return false
}
