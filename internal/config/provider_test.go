package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHardcodedZAIDefaults verifies that HardcodedZAIDefaults returns the
// expected "zai" provider with the correct built-in values.
func TestHardcodedZAIDefaults(t *testing.T) {
	pc := HardcodedZAIDefaults()

	if pc.DefaultProvider != "zai" {
		t.Errorf("DefaultProvider: got %q, want %q", pc.DefaultProvider, "zai")
	}

	p, ok := pc.Providers["zai"]
	if !ok {
		t.Fatal("expected 'zai' provider in Providers map")
	}

	if p.Name != "zai" {
		t.Errorf("Name: got %q, want %q", p.Name, "zai")
	}
	if p.BaseURL != ZaiBaseURL {
		t.Errorf("BaseURL: got %q, want %q", p.BaseURL, ZaiBaseURL)
	}
	if p.APIKeyFile != "~/.config/GoLeM/zai_api_key" {
		t.Errorf("APIKeyFile: got %q, want %q", p.APIKeyFile, "~/.config/GoLeM/zai_api_key")
	}
	if p.Models["opus"] != DefaultModel {
		t.Errorf("Models[opus]: got %q, want %q", p.Models["opus"], DefaultModel)
	}
	if p.Models["sonnet"] != DefaultModel {
		t.Errorf("Models[sonnet]: got %q, want %q", p.Models["sonnet"], DefaultModel)
	}
	if p.Models["haiku"] != DefaultModel {
		t.Errorf("Models[haiku]: got %q, want %q", p.Models["haiku"], DefaultModel)
	}
}

// TestParseProviderConfigEmpty verifies that empty/nil data returns
// HardcodedZAIDefaults.
func TestParseProviderConfigEmpty(t *testing.T) {
	// nil data
	pc, err := ParseProviderConfig(nil)
	if err != nil {
		t.Fatalf("ParseProviderConfig(nil) unexpected error: %v", err)
	}
	if pc.DefaultProvider != "zai" {
		t.Errorf("DefaultProvider: got %q, want %q", pc.DefaultProvider, "zai")
	}
	if _, ok := pc.Providers["zai"]; !ok {
		t.Error("expected 'zai' provider from hardcoded defaults")
	}

	// empty slice
	pc2, err := ParseProviderConfig([]byte{})
	if err != nil {
		t.Fatalf("ParseProviderConfig(empty) unexpected error: %v", err)
	}
	if pc2.DefaultProvider != "zai" {
		t.Errorf("DefaultProvider: got %q, want %q", pc2.DefaultProvider, "zai")
	}
}

// TestParseProviderConfigSingleProvider parses a TOML with one [providers.custom]
// section and verifies all fields are read correctly.
func TestParseProviderConfigSingleProvider(t *testing.T) {
	toml := `
[providers.custom]
base_url = "https://custom.api.com"
api_key_file = "/tmp/key"
timeout_ms = "5000"
opus_model = "custom-opus"
sonnet_model = "custom-sonnet"
haiku_model = "custom-haiku"
`
	pc, err := ParseProviderConfig([]byte(toml))
	if err != nil {
		t.Fatalf("ParseProviderConfig unexpected error: %v", err)
	}

	p, ok := pc.Providers["custom"]
	if !ok {
		t.Fatal("expected 'custom' provider in Providers map")
	}

	if p.Name != "custom" {
		t.Errorf("Name: got %q, want %q", p.Name, "custom")
	}
	if p.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL: got %q, want %q", p.BaseURL, "https://custom.api.com")
	}
	if p.APIKeyFile != "/tmp/key" {
		t.Errorf("APIKeyFile: got %q, want %q", p.APIKeyFile, "/tmp/key")
	}
	if p.TimeoutMs != "5000" {
		t.Errorf("TimeoutMs: got %q, want %q", p.TimeoutMs, "5000")
	}
	if p.Models["opus"] != "custom-opus" {
		t.Errorf("Models[opus]: got %q, want %q", p.Models["opus"], "custom-opus")
	}
	if p.Models["sonnet"] != "custom-sonnet" {
		t.Errorf("Models[sonnet]: got %q, want %q", p.Models["sonnet"], "custom-sonnet")
	}
	if p.Models["haiku"] != "custom-haiku" {
		t.Errorf("Models[haiku]: got %q, want %q", p.Models["haiku"], "custom-haiku")
	}
}

// TestParseProviderConfigMultipleProviders parses TOML with two [providers.*]
// sections and verifies both are present.
func TestParseProviderConfigMultipleProviders(t *testing.T) {
	toml := `
[providers.alpha]
base_url = "https://alpha.api.com"
api_key_file = "/tmp/alpha_key"

[providers.beta]
base_url = "https://beta.api.com"
api_key_file = "/tmp/beta_key"
`
	pc, err := ParseProviderConfig([]byte(toml))
	if err != nil {
		t.Fatalf("ParseProviderConfig unexpected error: %v", err)
	}

	if len(pc.Providers) != 2 {
		t.Errorf("Providers count: got %d, want 2", len(pc.Providers))
	}

	alpha, ok := pc.Providers["alpha"]
	if !ok {
		t.Fatal("expected 'alpha' provider")
	}
	if alpha.BaseURL != "https://alpha.api.com" {
		t.Errorf("alpha.BaseURL: got %q, want %q", alpha.BaseURL, "https://alpha.api.com")
	}

	beta, ok := pc.Providers["beta"]
	if !ok {
		t.Fatal("expected 'beta' provider")
	}
	if beta.APIKeyFile != "/tmp/beta_key" {
		t.Errorf("beta.APIKeyFile: got %q, want %q", beta.APIKeyFile, "/tmp/beta_key")
	}
}

// TestParseProviderConfigDefaultProvider verifies that default_provider key
// is read from the TOML and set on ProviderConfig.
func TestParseProviderConfigDefaultProvider(t *testing.T) {
	toml := `
default_provider = "custom"

[providers.custom]
base_url = "https://custom.api.com"
api_key_file = "/tmp/key"
`
	pc, err := ParseProviderConfig([]byte(toml))
	if err != nil {
		t.Fatalf("ParseProviderConfig unexpected error: %v", err)
	}

	if pc.DefaultProvider != "custom" {
		t.Errorf("DefaultProvider: got %q, want %q", pc.DefaultProvider, "custom")
	}
}

// TestLoadProviderHappyPath creates a temp dir with glm.toml containing a
// provider section and verifies LoadProvider returns it correctly.
func TestLoadProviderHappyPath(t *testing.T) {
	configDir := t.TempDir()
	toml := `
default_provider = "custom"

[providers.custom]
base_url = "https://custom.api.com"
api_key_file = "/tmp/key"
timeout_ms = "5000"
opus_model = "custom-opus"
sonnet_model = "custom-sonnet"
haiku_model = "custom-haiku"
`
	if err := os.WriteFile(filepath.Join(configDir, "glm.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write glm.toml: %v", err)
	}

	p, err := LoadProvider(configDir, "custom")
	if err != nil {
		t.Fatalf("LoadProvider unexpected error: %v", err)
	}

	if p.Name != "custom" {
		t.Errorf("Name: got %q, want %q", p.Name, "custom")
	}
	if p.BaseURL != "https://custom.api.com" {
		t.Errorf("BaseURL: got %q, want %q", p.BaseURL, "https://custom.api.com")
	}
}

// TestLoadProviderUnknown verifies that LoadProvider for a non-existent provider
// returns an error containing "not found".
func TestLoadProviderUnknown(t *testing.T) {
	configDir := t.TempDir()
	toml := `
[providers.custom]
base_url = "https://custom.api.com"
api_key_file = "/tmp/key"
`
	if err := os.WriteFile(filepath.Join(configDir, "glm.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write glm.toml: %v", err)
	}

	_, err := LoadProvider(configDir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should contain 'not found'; got: %s", err.Error())
	}
}

// TestLoadProviderFallbackToDefault verifies that LoadProvider with an empty
// name uses the DefaultProvider from the TOML.
func TestLoadProviderFallbackToDefault(t *testing.T) {
	configDir := t.TempDir()
	toml := `
default_provider = "myprovider"

[providers.myprovider]
base_url = "https://myprovider.api.com"
api_key_file = "/tmp/key"
`
	if err := os.WriteFile(filepath.Join(configDir, "glm.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write glm.toml: %v", err)
	}

	p, err := LoadProvider(configDir, "")
	if err != nil {
		t.Fatalf("LoadProvider unexpected error: %v", err)
	}
	if p.Name != "myprovider" {
		t.Errorf("Name: got %q, want %q (expected default provider)", p.Name, "myprovider")
	}
}

// TestLoadProviderNoTOML verifies that when no glm.toml exists, LoadProvider
// returns the hardcoded zai defaults.
func TestLoadProviderNoTOML(t *testing.T) {
	configDir := t.TempDir()
	// No glm.toml written.

	p, err := LoadProvider(configDir, "zai")
	if err != nil {
		t.Fatalf("LoadProvider unexpected error when no TOML: %v", err)
	}
	if p.Name != "zai" {
		t.Errorf("Name: got %q, want %q", p.Name, "zai")
	}
	if p.BaseURL != ZaiBaseURL {
		t.Errorf("BaseURL: got %q, want %q", p.BaseURL, ZaiBaseURL)
	}
}

// TestListProviders creates a glm.toml with 2 providers and verifies the
// returned list is sorted.
func TestListProviders(t *testing.T) {
	configDir := t.TempDir()
	toml := `
[providers.zebra]
base_url = "https://zebra.api.com"
api_key_file = "/tmp/key1"

[providers.alpha]
base_url = "https://alpha.api.com"
api_key_file = "/tmp/key2"
`
	if err := os.WriteFile(filepath.Join(configDir, "glm.toml"), []byte(toml), 0o644); err != nil {
		t.Fatalf("write glm.toml: %v", err)
	}

	names, err := ListProviders(configDir)
	if err != nil {
		t.Fatalf("ListProviders unexpected error: %v", err)
	}

	if len(names) != 2 {
		t.Fatalf("expected 2 provider names, got %d: %v", len(names), names)
	}
	if names[0] != "alpha" {
		t.Errorf("names[0]: got %q, want %q (must be sorted)", names[0], "alpha")
	}
	if names[1] != "zebra" {
		t.Errorf("names[1]: got %q, want %q (must be sorted)", names[1], "zebra")
	}
}

// TestListProvidersNoTOML verifies that when no glm.toml exists, ListProviders
// returns ["zai"].
func TestListProvidersNoTOML(t *testing.T) {
	configDir := t.TempDir()

	names, err := ListProviders(configDir)
	if err != nil {
		t.Fatalf("ListProviders unexpected error: %v", err)
	}

	if len(names) != 1 {
		t.Fatalf("expected 1 provider name, got %d: %v", len(names), names)
	}
	if names[0] != "zai" {
		t.Errorf("names[0]: got %q, want %q", names[0], "zai")
	}
}

// TestResolveModelEnv verifies the basic env map generation from a provider.
func TestResolveModelEnv(t *testing.T) {
	p := &Provider{
		Name:      "myprovider",
		BaseURL:   "https://my.api.com",
		TimeoutMs: "12345",
		Models: map[string]string{
			"opus":   "my-opus",
			"sonnet": "my-sonnet",
			"haiku":  "my-haiku",
		},
	}

	env := ResolveModelEnv(p, "test-api-key", "", "", "", "")

	if env["ANTHROPIC_BASE_URL"] != "https://my.api.com" {
		t.Errorf("ANTHROPIC_BASE_URL: got %q, want %q", env["ANTHROPIC_BASE_URL"], "https://my.api.com")
	}
	if env["API_TIMEOUT_MS"] != "12345" {
		t.Errorf("API_TIMEOUT_MS: got %q, want %q", env["API_TIMEOUT_MS"], "12345")
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "test-api-key" {
		t.Errorf("ANTHROPIC_AUTH_TOKEN: got %q, want %q", env["ANTHROPIC_AUTH_TOKEN"], "test-api-key")
	}
	if env["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "my-opus" {
		t.Errorf("ANTHROPIC_DEFAULT_OPUS_MODEL: got %q, want %q", env["ANTHROPIC_DEFAULT_OPUS_MODEL"], "my-opus")
	}
	if env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "my-sonnet" {
		t.Errorf("ANTHROPIC_DEFAULT_SONNET_MODEL: got %q, want %q", env["ANTHROPIC_DEFAULT_SONNET_MODEL"], "my-sonnet")
	}
	if env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "my-haiku" {
		t.Errorf("ANTHROPIC_DEFAULT_HAIKU_MODEL: got %q, want %q", env["ANTHROPIC_DEFAULT_HAIKU_MODEL"], "my-haiku")
	}
}

// TestResolveModelEnvWithOverrides verifies that modelOverride and per-slot
// overrides work correctly.
func TestResolveModelEnvWithOverrides(t *testing.T) {
	p := &Provider{
		Name:      "myprovider",
		BaseURL:   "https://my.api.com",
		TimeoutMs: "1000",
		Models: map[string]string{
			"opus":   "base-opus",
			"sonnet": "base-sonnet",
			"haiku":  "base-haiku",
		},
	}

	// modelOverride should override all three slots.
	env := ResolveModelEnv(p, "key", "override-all", "", "", "")
	if env["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "override-all" {
		t.Errorf("ANTHROPIC_DEFAULT_OPUS_MODEL with modelOverride: got %q, want %q", env["ANTHROPIC_DEFAULT_OPUS_MODEL"], "override-all")
	}
	if env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "override-all" {
		t.Errorf("ANTHROPIC_DEFAULT_SONNET_MODEL with modelOverride: got %q, want %q", env["ANTHROPIC_DEFAULT_SONNET_MODEL"], "override-all")
	}
	if env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "override-all" {
		t.Errorf("ANTHROPIC_DEFAULT_HAIKU_MODEL with modelOverride: got %q, want %q", env["ANTHROPIC_DEFAULT_HAIKU_MODEL"], "override-all")
	}

	// Per-slot override should win over modelOverride.
	env2 := ResolveModelEnv(p, "key", "override-all", "special-opus", "", "")
	if env2["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "special-opus" {
		t.Errorf("ANTHROPIC_DEFAULT_OPUS_MODEL with opusOverride: got %q, want %q", env2["ANTHROPIC_DEFAULT_OPUS_MODEL"], "special-opus")
	}
	if env2["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "override-all" {
		t.Errorf("ANTHROPIC_DEFAULT_SONNET_MODEL should use modelOverride when no per-slot: got %q, want %q", env2["ANTHROPIC_DEFAULT_SONNET_MODEL"], "override-all")
	}

	// Per-slot overrides without modelOverride.
	env3 := ResolveModelEnv(p, "key", "", "custom-opus", "custom-sonnet", "custom-haiku")
	if env3["ANTHROPIC_DEFAULT_OPUS_MODEL"] != "custom-opus" {
		t.Errorf("ANTHROPIC_DEFAULT_OPUS_MODEL: got %q, want %q", env3["ANTHROPIC_DEFAULT_OPUS_MODEL"], "custom-opus")
	}
	if env3["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "custom-sonnet" {
		t.Errorf("ANTHROPIC_DEFAULT_SONNET_MODEL: got %q, want %q", env3["ANTHROPIC_DEFAULT_SONNET_MODEL"], "custom-sonnet")
	}
	if env3["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "custom-haiku" {
		t.Errorf("ANTHROPIC_DEFAULT_HAIKU_MODEL: got %q, want %q", env3["ANTHROPIC_DEFAULT_HAIKU_MODEL"], "custom-haiku")
	}
}

// TestProviderAPIKey creates a temp file with a key and verifies Provider.APIKey()
// reads it correctly.
func TestProviderAPIKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "api_key")
	if err := os.WriteFile(keyPath, []byte("my-secret-key\n"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	p := &Provider{
		Name:       "test",
		APIKeyFile: keyPath,
	}

	key, err := p.APIKey()
	if err != nil {
		t.Fatalf("APIKey() unexpected error: %v", err)
	}
	if key != "my-secret-key" {
		t.Errorf("APIKey: got %q, want %q", key, "my-secret-key")
	}
}

// TestProviderAPIKeyMissing verifies that Provider.APIKey() returns an
// err:config error when the key file does not exist.
func TestProviderAPIKeyMissing(t *testing.T) {
	p := &Provider{
		Name:       "test",
		APIKeyFile: "/nonexistent/path/to/api_key",
	}

	_, err := p.APIKey()
	if err == nil {
		t.Fatal("expected error for missing key file, got nil")
	}
	if !strings.HasPrefix(err.Error(), "err:config") {
		t.Errorf("error should start with err:config; got: %s", err.Error())
	}
}

// TestExpandTilde verifies expandTilde handles "~/path", "~", and "/absolute/path".
func TestExpandTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}

	// "~/some/path" → home + "/some/path"
	result := expandTilde("~/some/path")
	want := home + "/some/path"
	if result != want {
		t.Errorf("expandTilde(~/some/path): got %q, want %q", result, want)
	}

	// "~" alone → home
	result2 := expandTilde("~")
	if result2 != home {
		t.Errorf("expandTilde(~): got %q, want %q", result2, home)
	}

	// absolute path is unchanged
	result3 := expandTilde("/absolute/path")
	if result3 != "/absolute/path" {
		t.Errorf("expandTilde(/absolute/path): got %q, want %q", result3, "/absolute/path")
	}

	// relative path without tilde is unchanged
	result4 := expandTilde("relative/path")
	if result4 != "relative/path" {
		t.Errorf("expandTilde(relative/path): got %q, want %q", result4, "relative/path")
	}
}
