# Next steps

## Implemented

- Embedded Go dashboard with a local-only default listener.
- Live Codex account, rate-limit, credit, earned-reset, and token-activity reads
  through the local app-server protocol and existing ChatGPT login.
- All returned limit buckets and daily activity rows, missing-value handling,
  per-section stale status, bounded polling, and shutdown cancellation.
- Protocol, partial-failure, cancellation, and rendering tests, plus standalone build instructions.

## Remaining

1. **Connect OpenRouter.** Confirm current official endpoints, authentication,
   scopes, and account-versus-key spend definitions. Add documented server-side
   credentials and a placeholder-only `.env.example`. Define currency and budget
   windows, keeping prepaid credits distinct from a configured monthly budget.
   Replace OpenRouter demo values with an explicit connection state and live data.
2. **Strengthen refresh behavior.** Add provider-aware retry guidance, backoff,
   and jitter. Consider a persistent Codex app-server connection if startup cost
   becomes significant. Preserve independent availability and never substitute
   fictional values for failed reads.
3. **Improve presentation.** Add browser-local timestamps and ticking countdowns;
   consider date filters for long daily-activity histories. Verify layout and
   accessibility in browsers and on mobile devices.
4. **Prepare for regular use.** Add CI for formatting, race tests, vet, and builds,
   plus service installation instructions and a documented CLI compatibility
   policy. Keep the default local listener; add authentication and HTTPS guidance
   before remote access is supported.
5. **Optional history.** Decide whether to store local snapshots for trends beyond
   the dates returned by Codex. Document retention and distinguish measured
   history from estimates. Per-thread billing exploration is a separate feature
   and is not needed for the account dashboard.
