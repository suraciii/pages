---
name: pages
description: Publish or share HTML pages and zip directory pages with the pages CLI. Use when the user wants to publish, upload, share, or update a report, artifact, dashboard, or web page; when they mention pages, a publish destination, a slug, or a public pages URL; or when the pages CLI may need installation.
---

# pages

Use the installed CLI as the source of truth for pages commands.

1. Run `pages skill`. If it succeeds, treat its stdout as the complete skill
   and follow it.
2. If `pages` is unavailable or does not support `skill`, install the current
   CLI:

```text literal
go install github.com/suraciii/pages@latest
```

3. Run `pages skill`.
4. Treat its stdout as the complete, authoritative skill for the installed
   CLI version and follow those instructions. Do not infer operational
   commands from this bootstrap skill.
