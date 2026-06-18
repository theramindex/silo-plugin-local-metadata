# Operations & Debugging

Operator-facing notes for `silo.local-metadata`.

## Request Lifecycle

The plugin only answers calls that include `GetMetadataRequest.file_path`.
Silo passes the media file path during metadata refreshes. The plugin derives
same-basename sidecar candidates from that path, parses the `.nfo` if present,
and returns any matching local artwork paths.

If no same-basename sidecar exists, `GetMetadata` returns an empty response
with no item. This is the normal "not found" signal and lets downstream
providers fill metadata.

## Common Workflows

### "My NFO was ignored"

1. Confirm the NFO uses the exact media basename:
   - `Movie.mkv` -> `Movie.nfo`
   - `Show - S01E02.mkv` -> `Show - S01E02.nfo`
2. Confirm the file is visible from the Silo container at the same path Silo
   stores for the media file.
3. Confirm the XML is well-formed. Malformed XML is returned as a plugin error
   so the bad sidecar is visible instead of silently ignored.
4. Trigger a metadata refresh for the item.

### "My poster was ignored"

1. Confirm the artwork uses the exact media basename.
2. Confirm the suffix is one of:
   - poster: `-poster`, `.poster`
   - backdrop: `-backdrop`, `.backdrop`, `-fanart`, `.fanart`
   - logo: `-logo`, `.logo`
   - still: `-thumb`, `.thumb`, `-still`, `.still`
3. Confirm the extension is `.png`, `.jpg`, `.jpeg`, or `.webp`.
4. Do not use folder-level assets like `poster.png`, `folder.jpg`, or
   `season01-poster.jpg`; this plugin intentionally ignores them.

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
