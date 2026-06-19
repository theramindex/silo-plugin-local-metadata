# Silo Local Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a standalone read-only Silo plugin that reads same-basename NFO and artwork sidecars beside media files.

**Architecture:** Implement a thin Silo SDK runtime wrapper around a testable sidecar package. The sidecar package owns path derivation, NFO XML parsing, image discovery, and image URL resolution. The runtime wrapper maps parsed fields into Silo `metadata_provider.v1` protobuf messages.

**Tech Stack:** Go 1.26, Silo plugin SDK v0.4.0, Go standard library XML parser, protobuf `structpb`.

---

### Task 1: Sidecar Discovery And NFO Parser

**Files:**
- Create: `internal/sidecar/sidecar.go`
- Test: `internal/sidecar/sidecar_test.go`

- [x] **Step 1: Write tests for same-basename NFO parsing**

Create a temp media file and matching `.nfo`, then assert title, year, runtime, provider IDs, ratings, and people are parsed.

- [x] **Step 2: Write tests for same-basename artwork only**

Create `Movie-poster.png` and generic `poster.png`, then assert only the same-basename image is returned.

- [x] **Step 3: Implement discovery and parser**

Use `filepath.Ext` to derive same-basename paths. Parse XML with `encoding/xml.Decoder` and ignore unknown fields.

- [x] **Step 4: Verify parser tests**

Run: `go test ./internal/sidecar`

### Task 2: Silo Runtime Wrapper

**Files:**
- Create: `main.go`
- Create: `provider/provider.go`
- Test: `main_test.go`

- [x] **Step 1: Write runtime mapping tests**

Assert `GetMetadata` maps NFO fields and poster paths into `MetadataItem`.

- [x] **Step 2: Implement runtime server**

Load embedded manifest, calculate binary checksum, serve runtime and metadata provider servers.

- [x] **Step 3: Implement metadata methods**

`GetMetadata`, `ResolveImageURL`, and `ResolveImageURLs` use sidecar data. `GetImages` returns an empty response because SDK v0.4.0 does not pass `file_path` to that request. Unsupported methods return empty responses.

- [x] **Step 4: Verify runtime tests**

Run: `go test ./...`

### Task 3: Packaging Metadata And Docs

**Files:**
- Create: `manifest.json`
- Create: `README.md`
- Create: `docs/superpowers/specs/2026-06-18-silo-plugin-local-metadata-design.md`
- Create: `docs/superpowers/plans/2026-06-18-silo-plugin-local-metadata.md`

- [x] **Step 1: Add plugin manifest**

Declare `silo.local-metadata` with `metadata_provider.v1` capability id `local-metadata`.

- [x] **Step 2: Add README**

Document same-basename-only behavior and supported NFO fields.

- [x] **Step 3: Add design and plan docs**

Save the approved design and implementation checklist.

- [x] **Step 4: Verify build**

Run: `go test ./...` and `go build -ldflags "-X main.version=0.1.0" .`
