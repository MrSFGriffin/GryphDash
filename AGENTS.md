# Contribution guidance

## Required validation

For code or API changes, run the repository checks that apply to the change:

```sh
go fmt ./...
go test ./...
go vet ./...
go build -o bin/gryphdash .
node --check web/dashboard.js
git diff --check
```

With a C compiler available, also run `CGO_ENABLED=1 go test -race ./...`.
Use focused tests when possible, but do not skip the relevant unit or integration
checks. Keep provider reads covered with local fixtures; tests must not require a
live Codex login or third-party account.

## Browser QA

Browser automation is optional and is not part of the default coding validation.
Do not download or launch Puppeteer, Playwright, Chromium, Firefox, or similar
browser tooling unless the user explicitly requests browser testing. The user may
run `tests/browser.mjs` as their own QA workflow.

Do not commit browser binaries, npm installation directories, caches, screenshots,
or other temporary browser-test output.

## Frontend changes

Keep frontend behavior testable through the server and JSON widget API where
practical. Preserve safe text rendering and explicit unavailable/stale states.
When changing widget IDs or layout data, maintain compatibility with saved browser
layouts or document the migration.
