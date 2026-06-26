# ui

Interactive terminal wizard for the full migration flow.

```bash
rancher-migrate ui
```

Running `rancher-migrate` with **no arguments** in a TTY also launches the UI.

## Menu

| Item | Flow |
|------|------|
| **Full migration** | Local or S3 → inspect → cluster select → sanitize → optional restore |
| **Sanitize backup** | Local file only |
| **Pull from S3** | List keys → download → sanitize |
| **Inspect only** | Inventory tree |
| **Restore to cluster** | Existing sanitized file → copy + Restore CR + watch |

See [TUI wizard](../tui.md) for keyboard shortcuts and screens.
