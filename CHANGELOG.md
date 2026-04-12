# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.0.0] - 2026-04-11

### Added
- Repository scaffold: directory layout per `prd/00-MASTER-ARCHITECTURE.md §2`
- `CLAUDE.md`, `AGENTS.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`
- `go.mod` with module `github.com/karthikcodes/aetronyx`, Go 1.23
- `Makefile` with `dev`, `build`, `test`, `lint`, `fmt`, `clean`, `ui-install`, `ui-dev`, `ui-build` targets
- `.goreleaser.yaml` for macOS arm64 (M1 target, other platforms in M6)
- GitHub Actions CI (`.github/workflows/ci.yml`) and release (`.github/workflows/release.yml`) workflows
- `.golangci.yml` linter configuration
- `.editorconfig` for consistent editor settings
- Issue templates and PR template
- `main.go` stub (empty `main()`, real entry point built in M1)
