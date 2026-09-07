# Next steps

## Implemented

- Embedded Go dashboard with a local-only default listener.
- Live Codex account, rate-limit, credit, earned-reset, and token-activity reads
  through the local app-server protocol and existing ChatGPT login.
- All returned limit buckets and daily activity rows, missing-value handling,
  per-section stale status, bounded polling, and shutdown cancellation.
- GridStack widgets with a searchable picker, drag/resize editing, keyboard
  controls, browser-local layout persistence, and mobile adaptation.
- In-place metric refresh and ticking reset countdowns.
- Declarative embedded `widgets.json` catalog with groups, descriptions, stable
  IDs, and typed logic for scalar, limit-window, daily, reset-detail, timestamp,
  and future URL-backed widgets.
- Protocol, partial-failure, cancellation, widget API, and browser interaction
  tests, plus standalone build instructions.

## Remaining

1. **Connect OpenRouter.** Confirm current official endpoints, authentication,
   scopes, and account-versus-key spend definitions. Add documented server-side
   credentials and a placeholder-only `.env.example`. Define currency and budget
   windows, keeping prepaid credits distinct from a configured monthly budget.
   Add OpenRouter widgets with an explicit connection state and live data.
2. **Strengthen refresh behavior.** Add provider-aware retry guidance, backoff,
   and jitter. Consider a persistent Codex app-server connection if startup cost
   becomes significant. Preserve independent availability and never substitute
   fictional values for failed reads.
3. **Improve presentation.** Consider browser-local timestamps, date filters for
   long daily histories, and optional layout export/import or server-side sync.
   Broaden browser and assistive-technology testing.
4. **Prepare for regular use.** Add CI for formatting, race tests, vet, and builds,
   plus service installation instructions and a documented CLI compatibility
   policy. Keep the default local listener; add authentication and HTTPS guidance
   before remote access is supported.
5. **Optional history.** Decide whether to store local snapshots for trends beyond
   the dates returned by Codex. Document retention and distinguish measured
   history from estimates. Per-thread billing exploration is a separate feature
   and is not needed for the account dashboard.
