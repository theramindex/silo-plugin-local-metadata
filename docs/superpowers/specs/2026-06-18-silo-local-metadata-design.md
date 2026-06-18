# Silo Local Metadata Plugin Design

## Goal

Build a read-only Silo metadata provider plugin that reads same-basename NFO and artwork sidecars located beside a media file, so local curated metadata can win before remote providers fill gaps.

## Architecture

The plugin is a standalone Go module using `github.com/Silo-Server/silo-plugin-sdk`. It implements `metadata_provider.v1` with a high default priority for movies, series, seasons, and episodes. Runtime code stays thin; sidecar discovery and NFO parsing live in an internal package with unit tests.

## Sidecar Rules

The plugin reads only immediate same-basename sidecars derived from `GetMetadataRequest.file_path`.

Examples:

- `Movie.mkv` -> `Movie.nfo`
- `Movie.mkv` -> `Movie-poster.png`, `Movie-poster.jpg`, `Movie.poster.jpg`
- `Episode.mkv` -> `Episode-fanart.jpg`, `Episode-logo.png`, `Episode-thumb.jpg`

The plugin does not read parent-folder files such as `tvshow.nfo`, `folder.jpg`, `poster.png`, or `season01-poster.jpg`.

## Data Flow

`GetMetadata` uses the request `file_path`, parses same-basename `.nfo` when present, finds same-basename images, and returns a partial `MetadataItem`. Missing fields stay empty so Silo can merge remote provider data.

`GetMetadata` also attaches poster, backdrop, and logo sidecar paths when those files exist. `ResolveImageURL` resolves the plugin's internal `local-metadata://` paths to local `file://` URLs for host-side ingestion. SDK v0.4.0 does not pass `file_path` to `GetImages`, so `GetImages` returns an empty response in this version.

`Search`, person detail, seasons, and episodes return empty responses in the first version because sidecar metadata is file-path based.

## Error Handling

Missing sidecars produce empty responses, not errors. Malformed NFO produces a parse error so the host can surface the bad local metadata. Missing image files resolve to an empty URL.

## Upstream Readiness

The module has no RamIndex-specific paths, secrets, or service dependencies. README and tests document the same-basename-only behavior explicitly so the Silo team can decide whether to accept that narrow scope or extend it later.
