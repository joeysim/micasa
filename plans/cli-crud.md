<!-- Copyright 2026 Phillip Cloud -->
<!-- Licensed under the Apache License, Version 2.0 -->

# CLI CRUD for Core Entities

## Goal

Add a first-class CLI surface for full entity manipulation (create, read, update,
delete, restore) to match the practical data-management flows currently
available in the TUI.

## Scope

- New top-level entity commands: `micasa <entity> <subcommand>`
- Entities in scope:
  - `house`
  - `projects`
  - `quotes`
  - `maintenance`
  - `service-log`
  - `appliances`
  - `incidents`
  - `vendors`
  - `documents`
- Operations:
  - `list`
  - `add`
  - `edit`
  - `delete`
  - `restore` (for soft-deleted entities; not house)

## CLI UX

- Mutation payloads use JSON via `--data` (inline) or `--data-file`.
- `edit`/`delete`/`restore` take positional `<id>` where applicable.
- The existing `show` command remains the read path.
- Command output is intentionally quiet on success (Unix-style); print only
  primary identifiers for create/update operations.

## Implementation Notes

- Reuse existing `internal/data` validation and dependency checks by routing
  directly through store methods (`CreateX`, `UpdateX`, `DeleteX`, `RestoreX`).
- Use per-entity decoder functions to unmarshal JSON into typed models.
- House profile supports create/update only.
- Document mutations support metadata changes only (existing blob/update flows
  remain in current commands).

## Testing Strategy

- Add CLI integration-style tests in `cmd/micasa` using real SQLite files.
- Cover at least one full lifecycle test (`create -> update -> delete ->
  restore`) and entity-specific tests for constraints/required args.
- Verify behavior through CLI entry points (`executeCLI`), not by directly
  calling data-layer methods for assertions.
