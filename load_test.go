package configx

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type nestedConfig struct {
	Enabled bool `json:"enabled" env:"CFG_ENABLED"`
}

type testConfig struct {
	APIKey  string        `json:"api_key" env:"API_KEY"`
	Env     string        `json:"env" env:"CLI_ENV,ENV"`
	Timeout int           `json:"timeout" env:"CLI_TIMEOUT"`
	Wait    time.Duration `json:"wait" env:"CLI_WAIT"`
	Nested  nestedConfig  `json:"nested"`
}

func TestLoadPrecedence_DefaultsThenJSONThenEnv(t *testing.T) {
	t.Setenv("CLI_ENV", "dev")
	t.Setenv("CLI_TIMEOUT", "30")
	t.Setenv("CLI_WAIT", "5s")

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{
		"api_key":"from-json",
		"env":"staging",
		"timeout":20,
		"wait":2000000000,
		"nested":{"enabled":true}
	}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := testConfig{
		Env:     "prod",
		Timeout: 10,
		Wait:    time.Second,
		Nested: nestedConfig{
			Enabled: false,
		},
	}

	if err := Load(&cfg, Options{
		ConfigPath:         configPath,
		AllowMissingConfig: false,
	}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.APIKey != "from-json" {
		t.Fatalf("APIKey: got %q, want %q", cfg.APIKey, "from-json")
	}
	if cfg.Env != "dev" {
		t.Fatalf("Env: got %q, want %q", cfg.Env, "dev")
	}
	if cfg.Timeout != 30 {
		t.Fatalf("Timeout: got %d, want %d", cfg.Timeout, 30)
	}
	if cfg.Wait != 5*time.Second {
		t.Fatalf("Wait: got %v, want %v", cfg.Wait, 5*time.Second)
	}
	if !cfg.Nested.Enabled {
		t.Fatalf("Nested.Enabled: got false, want true")
	}
}

func TestLoadDotEnvOverrideModes(t *testing.T) {
	tmpDir := t.TempDir()
	dotEnvPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(dotEnvPath, []byte("API_KEY=from-dotenv\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	t.Run("no override keeps existing env", func(t *testing.T) {
		t.Setenv("API_KEY", "from-os")
		cfg := testConfig{}
		if err := Load(&cfg, Options{
			DotEnvPaths:    []string{dotEnvPath},
			DotEnvOverride: false,
		}); err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if cfg.APIKey != "from-os" {
			t.Fatalf("APIKey: got %q, want %q", cfg.APIKey, "from-os")
		}
	})

	t.Run("override replaces existing env", func(t *testing.T) {
		t.Setenv("API_KEY", "from-os")
		cfg := testConfig{}
		if err := Load(&cfg, Options{
			DotEnvPaths:    []string{dotEnvPath},
			DotEnvOverride: true,
		}); err != nil {
			t.Fatalf("Load returned error: %v", err)
		}
		if cfg.APIKey != "from-dotenv" {
			t.Fatalf("APIKey: got %q, want %q", cfg.APIKey, "from-dotenv")
		}
	})
}

func TestLoadDotEnvParsingEdgeCases(t *testing.T) {
	type dotEnvCfg struct {
		APIKey string `env:"API_KEY"`
		Spaced string `env:"SPACED"`
		Quoted string `env:"QUOTED"`
		Single string `env:"SINGLE"`
		Inline string `env:"INLINE"`
	}

	tmpDir := t.TempDir()
	dotEnvPath := filepath.Join(tmpDir, ".env")
	content := strings.Join([]string{
		"# full line comment",
		"NOT_A_PAIR",
		"=missing_key",
		"API_KEY=from-dotenv",
		"  SPACED = spaced value  ",
		`QUOTED="double-quoted"`,
		"SINGLE='single-quoted'",
		"INLINE=value # trailing comment is kept by this parser",
		"",
	}, "\n")
	if err := os.WriteFile(dotEnvPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg := dotEnvCfg{}
	if err := Load(&cfg, Options{
		DotEnvPaths: []string{dotEnvPath},
	}); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.APIKey != "from-dotenv" {
		t.Fatalf("APIKey: got %q, want %q", cfg.APIKey, "from-dotenv")
	}
	if cfg.Spaced != "spaced value" {
		t.Fatalf("Spaced: got %q, want %q", cfg.Spaced, "spaced value")
	}
	if cfg.Quoted != "double-quoted" {
		t.Fatalf("Quoted: got %q, want %q", cfg.Quoted, "double-quoted")
	}
	if cfg.Single != "single-quoted" {
		t.Fatalf("Single: got %q, want %q", cfg.Single, "single-quoted")
	}
	if cfg.Inline != "value # trailing comment is kept by this parser" {
		t.Fatalf("Inline: got %q, want %q", cfg.Inline, "value # trailing comment is kept by this parser")
	}
}

func TestLoadMissingExplicitConfigPathErrors(t *testing.T) {
	cfg := testConfig{}
	err := Load(&cfg, Options{
		ConfigPath:         filepath.Join(t.TempDir(), "missing.json"),
		AllowMissingConfig: false,
	})
	if err == nil {
		t.Fatalf("expected error for missing config path")
	}
	if !strings.Contains(err.Error(), "config file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, ErrConfigNotFound) {
		t.Fatalf("expected ErrConfigNotFound, got: %v", err)
	}
}

func TestLoadAllowsMissingConfigWhenConfigured(t *testing.T) {
	cfg := testConfig{}
	err := Load(&cfg, Options{
		ConfigPath:         filepath.Join(t.TempDir(), "missing.json"),
		AllowMissingConfig: true,
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestLoadDestinationValidation(t *testing.T) {
	cfg := testConfig{}
	if err := Load(cfg, Options{}); err == nil || !strings.Contains(err.Error(), "non-nil pointer") {
		t.Fatalf("expected non-nil pointer error, got: %v", err)
	} else if !errors.Is(err, ErrDestinationInvalid) {
		t.Fatalf("expected ErrDestinationInvalid, got: %v", err)
	}

	var nilCfg *testConfig
	if err := Load(nilCfg, Options{}); err == nil || !strings.Contains(err.Error(), "non-nil pointer") {
		t.Fatalf("expected non-nil pointer error, got: %v", err)
	} else if !errors.Is(err, ErrDestinationInvalid) {
		t.Fatalf("expected ErrDestinationInvalid, got: %v", err)
	}

	notStruct := 42
	if err := Load(&notStruct, Options{}); err == nil || !strings.Contains(err.Error(), "point to a struct") {
		t.Fatalf("expected pointer-to-struct error, got: %v", err)
	} else if !errors.Is(err, ErrDestinationInvalid) {
		t.Fatalf("expected ErrDestinationInvalid, got: %v", err)
	}
}

func TestLoadInvalidEnvValueReturnsFieldError(t *testing.T) {
	t.Setenv("CLI_TIMEOUT", "not-an-int")
	cfg := testConfig{}
	err := Load(&cfg, Options{})
	if err == nil {
		t.Fatalf("expected error for invalid env value")
	}
	if !strings.Contains(err.Error(), "Timeout") {
		t.Fatalf("expected error mentioning field name, got: %v", err)
	}
	if !errors.Is(err, ErrEnvInvalid) {
		t.Fatalf("expected ErrEnvInvalid, got: %v", err)
	}
}

func TestLoadUnsupportedFieldTypeErrors(t *testing.T) {
	type unsupported struct {
		Items []string `env:"ITEMS"`
	}
	t.Setenv("ITEMS", "a,b,c")
	cfg := unsupported{}
	err := Load(&cfg, Options{})
	if err == nil {
		t.Fatalf("expected unsupported type error")
	}
	if !strings.Contains(err.Error(), "unsupported field type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigFileRejectsUnsupportedExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(path, []byte("x: 1"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	var cfg testConfig
	err := loadConfigFile(path, &cfg)
	if err == nil {
		t.Fatalf("expected unsupported format error")
	}
	if !strings.Contains(err.Error(), "unsupported config format") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, ErrConfigUnsupportedFormat) {
		t.Fatalf("expected ErrConfigUnsupportedFormat, got: %v", err)
	}
}

func TestResolveConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	existing := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(existing, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if got := resolveConfigPath(existing, nil); got != existing {
		t.Fatalf("explicit path: got %q, want %q", got, existing)
	}

	if got := resolveConfigPath("", []string{"", existing}); got != existing {
		t.Fatalf("search path: got %q, want %q", got, existing)
	}
}

func ExampleLoad() {
	type cfg struct {
		Env string `json:"env" env:"CLI_ENV"`
	}

	tmpDir, err := os.MkdirTemp("", "configx-example-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")
	if err := os.WriteFile(configPath, []byte(`{"env":"staging"}`), 0o600); err != nil {
		panic(err)
	}

	c := cfg{Env: "prod"}
	if err := Load(&c, Options{ConfigPath: configPath}); err != nil {
		panic(err)
	}

	fmt.Println(c.Env)
	// Output: staging
}
