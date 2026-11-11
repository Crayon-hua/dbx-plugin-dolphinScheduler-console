# Third-party notices

`backend/sdk/sdk.go` and `backend/sdk/go.mod` are unmodified copies of the DBX Go plugin SDK.

- Source: https://github.com/t8y2/dbx/tree/bef20fdc2214c4af8fb0aff1ae08ce8a1a309c53/plugins/sdk/go/dbx-plugin-sdk
- Copyright: DBX project contributors
- License: Apache License 2.0 (included in `LICENSE`)

`schemas/manifest.schema.json` is the DBX plugin manifest schema from the same commit, under the same license.
The CLI scaffold was generated with `@dbx-app/plugin-cli@0.1.10` and substantially modified for this plugin.

`backend/tools.json` contains reviewed request metadata derived from the user-provided Apache DolphinScheduler OpenAPI specification (version label 3.3.0). No deployment servers, credentials or configuration were copied. Apache DolphinScheduler is an Apache Software Foundation project licensed under Apache-2.0.
