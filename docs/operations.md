# Operations & Debugging

Operator-facing notes for `silo.local-metadata`.

## Request Lifecycle

The plugin only answers calls that include `GetMetadataRequest.file_path`.
Silo passes the media file path during metadata refreshes. The plugin derives
Jellyfin-compatible sidecar candidates from that path, parses the `.nfo` if
present, and returns any matching local artwork paths.

If no local sidecar exists, `GetMetadata` returns an empty response with no
item. This is the normal "not found" signal and lets downstream providers fill
metadata.

## Common Workflows

### "My NFO was ignored"

1. Confirm the NFO uses a supported Jellyfin-compatible name:
   - `Movie.mkv` -> `Movie.nfo`
   - `Movie Folder/Movie.mkv` -> `Movie Folder/movie.nfo`
   - `Show - S01E02.mkv` -> `Show - S01E02.nfo`
   - `Example Show/` -> `Example Show/tvshow.nfo`
   - `Season 01/` -> `Season 01/season.nfo`
2. Confirm the file is visible from the Silo container at the same path Silo
   stores for the media file.
3. Confirm the XML is well-formed. Malformed XML is returned as a plugin error
   so the bad sidecar is visible instead of silently ignored.
4. Trigger a metadata refresh for the item.

### "My poster was ignored"

1. Confirm the artwork uses either the media basename or a supported folder-level
   Jellyfin name.
2. Confirm the suffix is one of:
   - poster: `-poster`, `.poster`
   - backdrop: `-backdrop`, `.backdrop`, `-fanart`, `.fanart`
   - logo: `-logo`, `.logo`
   - still: `-thumb`, `.thumb`, `-still`, `.still`
3. Confirm the extension is `.png`, `.jpg`, `.jpeg`, or `.webp`.
4. Folder-level `poster.png`, `folder.jpg`, `fanart.jpg`, `backdrop.png`,
   `logo.png`, and `thumb.jpg` are supported. Season-specific names like
   `season01-poster.jpg` are not implemented yet.

## Provider IDs

The plugin stores a deterministic `ProviderIDs["local-metadata"]` value derived
from the media path. NFO-provided external IDs such as `imdbid`, `tmdbid`, and
`tvdbid` are also passed through when present.

## Where To Look In The Code

| Concern | File |
| --- | --- |
| Silo RPC wiring and proto translation | `main.go` |
| Sidecar path derivation and NFO parser | `internal/sidecar/sidecar.go` |
| Thin provider facade | `provider/provider.go` |
| Manifest metadata and category | `manifest.json` |
