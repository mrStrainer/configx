# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-27

### Added

- First public release of `configx`:
  - default -> file -> env precedence model
  - JSON config loading
  - `.env` file loading with optional override mode
  - typed environment parsing (`string`, `bool`, ints, uints, floats, `time.Duration`)
- Initial test coverage for config loading, env overrides, and `.env` parsing behavior.
- pkg.go.dev example test for `Load`.
- GitHub Actions CI workflow running `go test` and `go vet` on push/PR.
- `.gitignore` for common Go/local artifacts.
