# Next steps

1. **Confirm data sources and metric definitions.** Check current official Codex
   and OpenRouter documentation and the intended account types. Verify supported
   authentication, endpoints, permissions, and response fields before promising
   live metrics. Distinguish Codex subscription usage from OpenAI API billing;
   do not assume an OpenAI API key exposes Codex subscription limits. If a desired
   metric has no supported interface, show it as unavailable and document that
   limitation. Define reporting windows, reset times, currencies, and whether
   budgets apply to an account, a key, or a locally configured allowance.
2. **Add server-side configuration.** Introduce documented environment variables
   for supported credentials, refresh intervals, and an optional monthly budget.
   Add a placeholder-only `.env.example` once credential requirements are known.
   Validate settings at startup; keep secrets out of HTML, logs, and source control.
3. **Implement provider adapters.** Keep fetching separate from rendering behind
   a small common snapshot model. Start with OpenRouter spend and credits, then
   supported Codex metrics. Use explicit HTTP timeouts, handle authentication and
   rate-limit errors, and test adapters against local HTTP fixtures. Make demo
   mode explicit; never substitute dummy values for a failed live request.
4. **Cache and refresh snapshots.** Fetch on a configurable schedule rather than
   every page view. Respect provider rate limits and retry guidance. Track last
   successful updates and retain stale data with a visible status when a provider
   fails. Let one provider remain usable if the other is unavailable.
5. **Refine the dashboard.** Show connection state, update timestamps, reporting
   windows, and reset times. Distinguish prepaid credit balance from budget
   remaining. Add accessible usage indicators and optional lightweight refresh
   without introducing a frontend build requirement.
6. **Prepare for regular use.** Add handler and configuration tests, CI for Go
   formatting/tests/vet/build, graceful shutdown, and service installation notes.
   Keep the default local listener; document authentication and HTTPS requirements
   before supporting remote access to real account metrics.
