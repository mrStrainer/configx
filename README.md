# configx

Small config loader for Go CLIs with explicit precedence:

1. defaults already set in the destination struct
2. config file values (`.json`)
3. env overrides via struct tags (`env:"..."`)

Overall precedence:

- with `DotEnvOverride: false`: `defaults -> config file -> OS env -> .env-derived vars`
- with `DotEnvOverride: true`: `defaults -> config file -> .env-derived vars -> OS env (overridden by .env)`

Features:

- `.env` loading from one or more paths
- optional `.env` override mode (`DotEnvOverride`)
- config path search fallback list
- typed env parsing (`string`, `bool`, ints, uints, floats, `time.Duration`)

`.env` parsing notes (current behavior):

- full-line comments (`# ...`) are ignored
- invalid lines without `=` are ignored
- surrounding single/double quotes are trimmed from values
- inline `#` comments are not stripped (everything after `=` is treated as value text)
- when a field uses `env:"A,B"`, the first non-empty env var in order is used

## Required vs optional fields

`configx` only handles loading/merging. It does not enforce required fields.

Recommended pattern:

1. set defaults in code
2. call `configx.Load(...)`
3. run app-specific validation (`Validate() error`)

```go
func (c Config) Validate() error {
	if c.APIKey == "" {
		return errors.New("API_KEY is required")
	}
	return nil
}
```

Treat a field as optional when an empty value is acceptable or a default is sufficient. Treat it as required when the app cannot run safely/correctly without it.

## Error handling

`configx` exposes typed sentinel errors so callers can categorize failures:

- `ErrConfigNotFound`
- `ErrConfigRead`
- `ErrConfigParse`
- `ErrConfigUnsupportedFormat`
- `ErrDotEnvRead`
- `ErrDotEnvSet`
- `ErrDestinationInvalid`
- `ErrEnvInvalid`

Use `errors.Is(err, configx.ErrX)` in your app and map to your CLI policy (exit codes, hints, logging).

## Usage

```go
type Config struct {
	APIKey  string `json:"api_key" env:"API_KEY"`
	Env     string `json:"env" env:"CLI_ENV,ENV"`
	Timeout int    `json:"timeout" env:"CLI_TIMEOUT"`
}

cfg := Config{
	Env: "Prod", // defaults
}

err := configx.Load(&cfg, configx.Options{
	ConfigPath:         "", // optional explicit path
	SearchPaths:        []string{"config.json"},
	DotEnvPaths:        []string{".env"},
	DotEnvOverride:     false, // true => .env wins over existing OS env
	AllowMissingConfig: true,
})
if err != nil {
	panic(err)
}
```
