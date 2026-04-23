# Language Server Protocol (LSP) support

`elastic-package` ships a built-in LSP server for authoring integration packages. It provides real-time diagnostics, context-aware completions, and hover documentation — all driven by the official [package-spec](https://github.com/elastic/package-spec) schema and the package's own field definitions.

## Starting the server

```bash
elastic-package lsp
```

The server speaks LSP over stdin/stdout and can be wired up to any LSP-capable editor (VS Code, Neovim, Helix, etc.).

## Features

### Diagnostics

The server validates the entire package against the package-spec schema as you edit. Errors are mapped to their source files and appear inline as squiggles or problem-list entries. Diagnostics clear automatically when the error is fixed.

Validation is debounced so it doesn't fire on every keystroke — only after a short pause.

### Completions

**Manifest keys and values** (`manifest.yml`)

When editing any `manifest.yml`, the server suggests valid keys at the current YAML path and enum values where applicable (e.g. `type:`, `format:`). The suggestions are schema-driven and adapt to the manifest type: integration, input, data-stream, or content.

**Field names in pipelines** (`elasticsearch/ingest_pipeline/*.yml`)

When the cursor is after `field:`, `target_field:`, `source:`, or `copy_to:`, the server suggests all field names defined across the package's `fields/*.yml` files. Each suggestion shows the field's type, unit, and metric type inline:

```
event.duration    long, unit: nanos, metric: counter
http.response.bytes    long, unit: byte, metric: counter
```

Completions are scoped to the data stream the file belongs to when possible.

**Field types in field definitions** (`fields/*.yml`)

When editing a `type:` key in a field definition file, the server suggests all valid Elasticsearch field types (`keyword`, `text`, `long`, `geo_point`, `nested`, etc.).

### Hover documentation

**Field references in pipelines**

Hovering over a field name after `field:` / `target_field:` / etc. shows the field's full metadata:

```
event.duration  long

Total duration of the event in nanoseconds.

Unit: nanos
Metric type: counter
```

**Manifest keys**

Hovering over a key in `manifest.yml` shows its schema description. The server resolves the full YAML path by walking up the indentation, so nested keys get the right docs.

**Field type / unit / metric_type values**

Hovering over a value like `scaled_float`, `nanos`, or `counter` in a `fields/*.yml` file shows a short inline reference:

- Field types: what the type means, storage characteristics, query limitations.
- Units: `byte`, `percent`, `ms`, `nanos`, etc.
- Metric types: `counter` vs `gauge` semantics.

## Editor setup

Any editor with LSP support can use this server. Point your LSP client at `elastic-package lsp` with no arguments. The server uses the `stdio` transport.

Example VS Code `settings.json` snippet (using the generic [custom LSP extension](https://marketplace.visualstudio.com/items?itemName=genericlanguageserver.custom-lsp)):

```json
{
  "customLsp.servers": [
    {
      "name": "elastic-package",
      "command": ["elastic-package", "lsp"],
      "filetypes": ["yaml"]
    }
  ]
}
```
