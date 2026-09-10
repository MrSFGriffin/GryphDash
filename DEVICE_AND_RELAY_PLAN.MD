# GryphDash realtime relay and device access

## Summary

Add a separately deployable relay service while keeping the existing local
GryphDash app responsible for provider access.

- `gryphdash`: local collector, browser dashboard, TUI, and outbound publisher.
- `gryphdash-relay`: public authenticated gateway for mobile and watch clients.
- Shared protocol and storage packages live in the same repository.
- The relay can run on a VPS, home machine, container host, or cloud VM without
  changing clients.
- No Firebase, Supabase, AWS IoT, or hosted database is required.

The local app remains the only component holding Codex/OpenRouter credentials.
The relay stores and forwards metric snapshots.

## Realtime data flow

```text
Codex/OpenRouter
       │
       ▼
local GryphDash publisher
       │ outbound TLS WebSocket
       ▼
GryphDash relay
       │ authenticated WebSocket
       ▼
Android phone ── Wear OS Data Layer ──> watch
```

The publisher sends a complete snapshot immediately after connecting, then
sends updates whenever collector state changes. The relay sends the latest
snapshot immediately when a device connects, followed by realtime updates.
Each snapshot includes a schema version, timestamp, sequence number, provider
status, and stale/error information.

The first client protocol is a versioned WebSocket protocol. MQTT is not
exposed directly to mobile clients; a future MQTT adapter can be added behind
the relay if it becomes useful.

## Pairing and credential lifecycle

The local GryphDash publisher requests a pairing session from the relay and
displays a short-lived code or QR payload. The phone connects to the relay,
submits the code, and receives its own device credentials.

- Pairing codes are single-use and expire after approximately 5–10 minutes.
- Publisher credentials and device credentials are separate.
- Device access tokens are short-lived, approximately 15–60 minutes.
- Device refresh tokens are long-lived, securely stored, rotated on use, and
  valid for at least 90 days of inactivity.
- Re-pairing is not required after a normal holiday or several days without
  access to the local agent.
- Devices can be individually named, listed, revoked, and re-paired.
- If the publisher is offline, the relay continues serving the last snapshot
  and marks its age; it does not fabricate fresh values.

The relay must reject expired, replayed, account-mismatched, and revoked
credentials.

## Relay service boundaries

Define shared versioned types for:

- Publisher handshake and authentication.
- Device handshake and authentication.
- Pairing session creation and exchange.
- Full snapshots and update messages.
- Replay/resynchronization after reconnect.
- Sequence numbers, timestamps, schema versions, and stale states.
- Credential revocation and device metadata.

Define interfaces for:

- Publisher session handling.
- Device session fan-out.
- Latest-snapshot storage.
- Pairing and credential storage.

Start with an atomic filesystem-backed store containing the latest snapshot and
account/device state. Keep storage behind interfaces so SQLite, Postgres,
object storage, or another backend can be introduced later without changing
the mobile protocol.

## Repository and deployment structure

Use two commands in the same repository:

```text
cmd/gryphdash/
cmd/gryphdash-relay/
internal/protocol/
internal/storage/
external provider executables/
```

Provider clients remain outside the relay. The relay must not import provider
clients or receive provider credentials.

The local application gains:

- Relay URL configuration.
- Publisher credential configuration.
- A publisher connection manager with reconnect and backoff.
- Snapshot serialization and change detection.
- Pairing-code display or QR payload generation.
- A clear disconnected/stale status in the local dashboard and TUI.

The relay gains:

- TLS WebSocket endpoint.
- Publisher and device authentication.
- Pairing and device management.
- Snapshot persistence and replay.
- Per-account connection fan-out.
- Rate limits, payload-size limits, idle timeouts, and structured audit logging.

## Android and watch clients

Build the Android phone client against the shared WebSocket protocol.

The phone should:

- Pair using the one-time code or QR payload.
- Store refresh credentials in Android Keystore-backed secure storage.
- Maintain a realtime connection while active.
- Reconnect with backoff and request the latest snapshot after reconnect.
- Display snapshot age and provider errors.
- Continue showing the last authenticated snapshot while the publisher is
  offline.

The initial Wear OS client should receive a compact metric model from the phone
through the Android Data Layer. It should not require separate cloud
credentials or maintain an independent long-lived cloud connection.

## Security and privacy

- Require TLS for all relay connections.
- Never send Codex or OpenRouter provider tokens to the relay or devices.
- Scope every snapshot and credential to an account.
- Prevent cross-account fan-out and storage access.
- Rotate refresh credentials and support explicit revocation.
- Do not log metric payloads or credentials by default.
- Support account/device deletion before production rollout.
- Include snapshot age and source status in every client-visible data model.

## Test plan

Add tests for:

- Publisher authentication and reconnect behavior.
- Device authentication, refresh, expiry, and revocation.
- Pairing creation, expiry, single-use behavior, and account binding.
- Immediate full snapshot delivery on device connection.
- Realtime fan-out to multiple devices.
- Reconnect and latest-snapshot replay.
- Out-of-order and stale update rejection.
- Atomic persistence and relay restart recovery.
- Multiple accounts and multiple devices with isolation.
- Malformed, oversized, unauthorized, and timed-out WebSocket messages.
- Relay unavailability from the publisher’s perspective.
- No provider credentials in persisted state, device messages, or logs.
- Android secure credential storage and watch synchronization.

## Assumptions

- Android is the first native client; Wear OS follows through the phone.
- Realtime target is under five seconds when publisher and device connections
  are healthy.
- The relay is trusted to read metric values.
- The relay stores only the latest snapshot by default.
- One account can have multiple phones and watches.
- Deployment infrastructure is replaceable and outside the protocol design.
- Pairing is an initial device-registration flow; it is not required for routine
  reauthentication.
