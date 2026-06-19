# Silo Local Metadata

`silo.local-metadata` is a read-only metadata provider plugin that exposes
same-basename NFO files and local artwork sidecars to Silo's
`metadata_provider.v1` pipeline.

Repository: https://github.com/theramindex/silo-plugin-local-metadata

## Category

Lives under **Video / Metadata** (`category: "Video/Metadata"` in
`manifest.json`).

## Capabilities

| Type | ID | Purpose |
| --- | --- | --- |
| `metadata_provider.v1` | `local-metadata` | Reads same-basename `.nfo` metadata and local poster/backdrop/logo/still artwork beside the media file. Default priority `1` for movie / series / season / episode. |

## Dependencies

Standalone. The plugin is consumed directly by the Silo host's metadata
pipeline alongside other metadata providers such as TMDB and TVDB. It has no
SPA, no external network dependency, no library catalog of its own, and no
playback wiring.

## Sidecar Rules

The plugin is intentionally narrow and read-only:

- `Movie.mkv` may read `Movie.nfo`
- `Movie.mkv` may read `Movie-poster.png`, `Movie-poster.jpg`,
  `Movie.poster.jpg`, and equivalent `jpeg`/`webp` files
- Backdrops use same-basename `-backdrop`, `.backdrop`, `-fanart`, or `.fanart`
- Logos use same-basename `-logo` or `.logo`
- Stills use same-basename `-thumb`, `.thumb`, `-still`, or `.still`
- It does not read parent folder metadata such as `tvshow.nfo`, `folder.jpg`,
  `poster.png`, or `season01-poster.jpg`
- It does not write or modify media folders

## Supported NFO Fields

The parser accepts common Kodi/Plex-style XML fields:

- `title`, `originaltitle`, `sorttitle`
- `plot` or `outline`
- `tagline`, `year`, `runtime`
- `genre`, `studio`, `country`
- `mpaa`, `certification`, `contentrating`
- `premiered`, `releasedate`, `aired`
- `imdbid`, `tmdbid`, `tvdbid`
- `rating` / `userrating`
- `actor`, `director`, `writer`

Unknown fields are ignored. Missing fields are left empty so downstream
providers can fill gaps.

## Image URL Plumbing

Local sidecar images are returned as opaque `local-metadata://<absolute-path>`
URIs. When the host asks to resolve the image, the plugin validates that the
file still exists and returns a `file://` URL for host-side ingestion.

SDK v0.7.0 does not pass `file_path` to `GetImages`, so sidecar images are
attached through `GetMetadata` fields (`poster_path`, `backdrop_path`,
`logo_path`) rather than the standalone image listing RPC.

## Build And Release

Local build targets:

```sh
make build       # single binary for the host arch
make build-all   # linux/amd64, linux/arm64, darwin/arm64 into dist/
make checksums
make test
make lint
```

The intended release catalog is:

```text
https://github.com/theramindex/silo-plugins
```
