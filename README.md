# dl-daemon

`dl-daemon` is a Go-based downloader daemon for sources like Chzzk VOD/live and Anilife.

Right now the project is in an early scaffold stage with the first Chzzk VOD provider being wired in.

## Current architecture

- `cmd/dld` — daemon entrypoint
- `internal/manager` — polling/watch loop and active download session management
- `internal/db` — sqlite persistence for targets and downloads
- `internal/platform` — provider abstraction
- `internal/platform/chzzk` — Chzzk VOD provider implementation

## Current status

Implemented:
- daemon lifecycle
- sqlite-backed target/download tracking
- provider/session abstraction
- initial Chzzk VOD provider scaffold

Still needed:
- target management CLI
- config/auth plumbing
- richer progress reporting
- live stream support
- Anilife provider

## Build

```bash
make build
```

Binary output:

```bash
bin/dld
```

## Development

Run tests:

```bash
go test ./...
```

## Notes

The project is being refactored from source-specific downloader code into a provider-based daemon architecture.
