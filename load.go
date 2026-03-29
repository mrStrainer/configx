package configx

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrConfigNotFound indicates an explicit config path could not be found.
	ErrConfigNotFound = errors.New("config file not found")
	// ErrConfigRead indicates a config file could not be read.
	ErrConfigRead = errors.New("config file read failed")
	// ErrConfigParse indicates config content could not be parsed.
	ErrConfigParse = errors.New("config parse failed")
	// ErrConfigUnsupportedFormat indicates an unsupported config extension.
	ErrConfigUnsupportedFormat = errors.New("config format not supported")
	// ErrDotEnvRead indicates a .env file could not be read.
	ErrDotEnvRead = errors.New("dotenv read failed")
	// ErrDotEnvSet indicates setting an environment variable failed.
	ErrDotEnvSet = errors.New("dotenv setenv failed")
	// ErrDestinationInvalid indicates the destination for loading is invalid.
	ErrDestinationInvalid = errors.New("invalid destination")
	// ErrEnvInvalid indicates an env var value could not be converted to target type.
	ErrEnvInvalid = errors.New("env value invalid")
)

type Options struct {
	ConfigPath         string
	SearchPaths        []string
	DotEnvPaths        []string
	DotEnvOverride     bool
	AllowMissingConfig bool
}

// Load merges values in this order:
// 1) existing defaults in dst
// 2) config file (if found)
// 3) environment variable overrides via `env` tags
func Load(dst any, opts Options) error {
	if err := loadDotEnvFiles(opts.DotEnvPaths, opts.DotEnvOverride); err != nil {
		return err
	}

	configPath := resolveConfigPath(opts.ConfigPath, opts.SearchPaths)
	if configPath != "" {
		if err := loadConfigFile(configPath, dst); err != nil {
			return err
		}
	} else if !opts.AllowMissingConfig && opts.ConfigPath != "" {
		return fmt.Errorf("%w: %s", ErrConfigNotFound, opts.ConfigPath)
	}

	if err := applyEnvOverrides(dst); err != nil {
		return err
	}

	return nil
}

func loadDotEnvFiles(paths []string, override bool) error {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if err := loadDotEnv(path, override); err != nil {
			return err
		}
	}
	return nil
}

func loadDotEnv(path string, override bool) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%w: opening %s: %v", ErrDotEnvRead, path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}

		if override || os.Getenv(key) == "" {
			if err := os.Setenv(key, strings.Trim(value, `"'`)); err != nil {
				return fmt.Errorf("%w: setting env var %s from %s: %v", ErrDotEnvSet, key, path, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("%w: reading %s: %v", ErrDotEnvRead, path, err)
	}
	return nil
}

func resolveConfigPath(explicitPath string, searchPaths []string) string {
	if strings.TrimSpace(explicitPath) != "" {
		if _, err := os.Stat(explicitPath); err == nil {
			return explicitPath
		}
		return ""
	}

	for _, candidate := range searchPaths {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		for _, candidate := range searchPaths {
			if strings.TrimSpace(candidate) == "" {
				continue
			}
			fullPath := filepath.Join(exeDir, candidate)
			if _, statErr := os.Stat(fullPath); statErr == nil {
				return fullPath
			}
		}
	}

	return ""
}

func loadConfigFile(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%w: reading config file %s: %v", ErrConfigRead, path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		if err := json.Unmarshal(data, dst); err != nil {
			return fmt.Errorf("%w: parsing JSON config %s: %v", ErrConfigParse, path, err)
		}
	default:
		return fmt.Errorf("%w: unsupported config format %q for %s (only .json is supported)", ErrConfigUnsupportedFormat, ext, path)
	}
	return nil
}

func applyEnvOverrides(dst any) error {
	v := reflect.ValueOf(dst)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return fmt.Errorf("%w: destination must be a non-nil pointer", ErrDestinationInvalid)
	}

	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return fmt.Errorf("%w: destination must point to a struct", ErrDestinationInvalid)
	}

	return applyEnvToStruct(elem)
}

func applyEnvToStruct(v reflect.Value) error {
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fieldValue := v.Field(i)
		if !fieldValue.CanSet() {
			continue
		}

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0)) {
			if err := applyEnvToStruct(fieldValue); err != nil {
				return err
			}
			continue
		}

		envTag := strings.TrimSpace(field.Tag.Get("env"))
		if envTag == "" {
			continue
		}

		value, ok := firstEnvValue(envTag)
		if !ok {
			continue
		}

		if err := setFromString(fieldValue, value); err != nil {
			return fmt.Errorf("%w: invalid env value for %s: %v", ErrEnvInvalid, field.Name, err)
		}
	}
	return nil
}

func firstEnvValue(tag string) (string, bool) {
	names := strings.Split(tag, ",")
	for _, raw := range names {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if value := os.Getenv(name); value != "" {
			return value, true
		}
	}
	return "", false
}

func setFromString(field reflect.Value, value string) error {
	kind := field.Kind()
	switch kind {
	case reflect.String:
		field.SetString(value)
		return nil
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		field.SetBool(parsed)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if field.Type() == reflect.TypeOf(time.Duration(0)) {
			dur, err := time.ParseDuration(value)
			if err != nil {
				return err
			}
			field.SetInt(int64(dur))
			return nil
		}
		parsed, err := strconv.ParseInt(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(parsed)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(parsed)
		return nil
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(value, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(parsed)
		return nil
	default:
		return fmt.Errorf("unsupported field type %s", field.Type().String())
	}
}
