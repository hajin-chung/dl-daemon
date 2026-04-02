# dl-daemon

`dl-daemon` is a Go-based downloader daemon for sources like Chzzk VOD/live and Anilife.

Right now the project is in an early but working stage with Chzzk VOD downloading wired into the daemon.

## Current architecture

- `cmd/dld` — CLI and daemon entrypoint
- `internal/manager` — polling/watch loop and active download session management
- `internal/db` — sqlite persistence for config, targets, and downloads
- `internal/platform` — provider abstraction
- `internal/platform/chzzk` — Chzzk VOD provider implementation
- `internal/logging` — dual logging setup (pretty console + JSON file)

## Current features

Implemented:
- daemon lifecycle
- sqlite-backed target/download tracking
- provider/session abstraction
- Chzzk VOD provider
- structured console and file logging
- config storage in the metadata table
- Chzzk token validation command

Still needed:
- watch-once/debug command
- richer progress semantics
- retry/backoff hardening
- Chzzk live provider
- Anilife provider
- richer target configuration

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

## CLI

Top-level commands:

```bash
dld run
dld target <subcommand>
dld config <subcommand>
dld chzzk <subcommand>
dld downloads
```

### Target commands

```bash
dld target add <platform> <id> [label]
dld target list
dld target remove <platform> <id>
```

Example:

```bash
dld target add chzzk a02dc370efd2befeac97881dc83f11bb tsuyu
```

### Config commands

```bash
dld config set <key> <value>
dld config get <key>
dld config list
```

Examples:

```bash
dld config set chzzk.nid_aut <value>
dld config set chzzk.nid_ses <value>
```

Note: `dld config list` masks sensitive values when printing.

### Chzzk commands

```bash
dld chzzk me
```

Checks whether the configured Chzzk auth tokens are valid.

### Downloads

```bash
dld downloads
```

Lists tracked downloads and their current status.

## Logging

`dld` logs in two places:

- pretty text logs to stderr
- JSON logs to `/home/claw/.config/dld/logs/dld.log.jsonl`

Environment variables:

```bash
DLD_LOG_LEVEL=debug
aDLD_FILE_LOG_LEVEL=debug
```

Common levels:
- `debug`
- `info`
- `warn`
- `error`

## Notes

The project is being refactored from source-specific downloader code into a provider-based daemon architecture.
