# Validation record — v0.1.0

Date: 2026-11-11. Target: Windows x64, DBX Sidecar Protocol v1.

Passed locally:

- Go tests covering all 31 reviewed API request paths/methods/parameters and form encoding.
- Read-only and explicit confirmation guards; refused writes send no HTTP request.
- Business errors, HTTP errors, invalid response shapes, oversized responses and redirect refusal.
- Token redaction in errors and returned strings; large integer precision preservation.
- Connection test/connect/disconnect including repeated lifecycle calls; MCP catalog and calls.
- Parameter types, unknown parameters, pagination bounds and connection configuration validation.
- `go vet ./...` and `node --check ui/app.js`.
- Official DBX manifest JSON Schema and tool declaration validation.
- Official `dbx-plugin` CLI 0.1.10 package creation and package contents inspection.
- Chromium + official DBX development runtime with the actual packaged Windows executable and a local HTTP test double: user-defined connection save/connect; projects/workflows; int64 values; pagination; read-only actions; instance/task/log flow; confirmed workflow start with empty scheduleTime; schedule details; all-tool forms; narrow layout; no browser script errors.

No user DolphinScheduler instance was accessed. Live compatibility, real Token permissions, enterprise gateway behavior and the actual desktop installation remain user acceptance checks. The local mock does not prove that an arbitrary DS version accepts every request.

The first CI run validates the checked-in source on Linux and Windows. The Release workflow builds Windows x64 in a clean runner. Those remote results are tracked in the repository Actions page.
