---
name: pages
description: Publish or share HTML pages and zip directory pages with the pages CLI. Use when the user wants to publish, upload, share, or update a report, artifact, dashboard, or web page; when they mention pages, a publish destination, a slug, or a public pages URL; or when the pages CLI may need installation.
---

# pages

Use the installed CLI as the source of truth for pages commands.

1. Check for the CLI with `command -v pages`.
2. When it is available, run `pages skill`. If that succeeds, treat its
   stdout as the complete skill and follow it. If the command is unknown,
   continue below to upgrade the CLI.
3. Check for Go with `command -v go`. If Go is unavailable, tell the user
   that Go is required and stop. Otherwise run:

```text literal
go install github.com/suraciii/pages@latest
pages_bin_dir="$(go env GOBIN)"
if [ -z "$pages_bin_dir" ]; then pages_bin_dir="$(go env GOPATH)/bin"; fi
export PATH="$pages_bin_dir:$PATH"
```

4. Run `pages skill`.
5. Treat its stdout as the complete, authoritative skill for the installed
   CLI version and follow those instructions. Do not infer operational
   commands from this bootstrap skill.
