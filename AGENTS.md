# AGENTS.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Overview

`cdnware` is a Go CLI tool that revisions static assets (adds content hash to filenames) and updates references in HTML/CSS/JS files for CDN deployment. Designed for Jekyll sites but works with any static site generator.

## Build & Run

```bash
just build                                  # Build binary
just test                                   # Run tests
./cdnware -cdn https://cdn.example.com _site  # Run against a site directory
./cdnware -cdn https://cdn.example.com -src static -dest static-rev _site  # Custom directories
```

## Release

Releases are automated via GoReleaser. Push a version tag to trigger:
```bash
git tag v1.0.0 && git push --tags
```

## Code Architecture

Single-file application (`cdnware.go`) with this flow:

1. **Asset revisioning** (`rev()` → `revFile()` → `hashFile()`):
   - Walks `<baseDir>/assets/` for supported file types
   - Creates MD5 hash (first 8 chars) of each file
   - Copies to `<baseDir>/assets-rev/` with hash in filename (e.g., `styles.1f81c53a.css`)

2. **Reference replacement** (`useman()` → `repFile()`):
   - Walks `<baseDir>` for .css, .js, .html, .webmanifest files (excluding `assets/`)
   - Replaces `/assets/...` references with CDN URLs from the manifest

3. **Output**: JSON manifest mapping original paths to CDN URLs

### Supported Asset Types

Configurable via `-asset-exts` or `asset_exts` in a config file (default: `.css`, `.js`, `.jpg`, `.png`, `.webp`, `.svg`, `.ico`, `.mp4`, `.woff2`, `.avif`). Source-file extensions scanned for refs are configurable via `-source-exts` / `source_exts` (default: `.css`, `.js`, `.html`, `.toml`, `.webmanifest`).

### Configuration

All settings (`cdn`, `src`, `dest`, `asset_exts`, `source_exts`, `hash_len`) can be set via CLI flags or a `cdnware.{toml,yaml,yml,json}` config file auto-discovered in SITEROOT or CWD. Precedence: flag > file > default. Use `-config <path>` to point at a specific file, or `-config -` to disable auto-discovery.

### Key Patterns

- Asset references in source files are matched by regex looking for paths in quotes or parentheses: `["'(]/<srcDir>/...["')]`
- The `-cdn` flag prepends a base URL to all revisioned paths
- The `-src` and `-dest` flags override the default `assets` and `assets-rev` directories
- The `-hash-len` flag (1–32) controls hex chars of MD5 hash embedded in filenames
