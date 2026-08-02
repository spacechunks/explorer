# JWK Fetch Extension for JWX

## Overview

This module (`github.com/jwx-go/jwkfetch/v4`) provides HTTP-based JWK Set
retrieval for `github.com/lestrrat-go/jwx/v4`. It offers two
complementary types, both of which implement `jwk.Fetcher`:

- **`Client`** — a one-shot HTTP JWKS fetcher. Defaults to permitting
  every URL: fine for compile-time-constant or trusted-config URLs.
  Set `WithWhitelist` to restrict — required when the URL comes from
  an untrusted source such as a JWS `jku` header, otherwise an
  attacker can point the fetcher at any URL it can reach (SSRF /
  key substitution).
- **`Cache`** — an httprc-backed store that keeps a fixed set of
  registered JWKS URLs hot with background refresh. Use it when you
  have a small, trusted list of JWKS endpoints and want to amortize
  fetch cost. Cache has no whitelist of its own; the URLs it will
  contact are exactly the ones you passed to `Register`.

This package holds the HTTP JWK Set fetch surface so the core jwx
`jwk` module has no dependency on `net/http` or `httprc`. `jwk`
itself only defines the abstract `Fetcher` interface; concrete
implementations live here.

## Architecture

Both `Client` and `Cache` are **closed structs** — all fields are
unexported, and construction is via functional options. Configuration
that is meaningful to both types is exposed as `GlobalFetchOption`
values (`WithHTTPClient`, `WithMaxBodySize`, `WithParseOptions`) that
satisfy both `ClientOption` and `CacheOption`. `WithWhitelist` is a
`ClientOption` only — `Cache` treats `Register` as the trust boundary
and does not enforce a whitelist, so passing `WithWhitelist` to
`NewCache` is a compile-time error.

`Cache` wraps an `httprc.Controller` and delegates background refresh
to httprc. An internal `transformer` converts each HTTP response to a
`jwk.Set` using the cache's configured body-size cap and parse
options.

### Key types

| Type | Purpose |
|------|---------|
| `Client` | One-shot HTTP JWKS fetcher; implements `jwk.Fetcher` |
| `Cache` | Background-refreshed JWKS store; implements `jwk.Fetcher` |
| `CachedSet` / `NewCachedSet` | Read-only `jwk.Set` view backed by an httprc resource (used internally by `Cache.CachedSet`) |
| `Whitelist`, `InsecureWhitelist`, `BlockAllWhitelist`, `MapWhitelist`, `RegexpWhitelist`, `WhitelistFunc` | URL allowlist policies consulted by `Client.Fetch` |
| `HTTPClient` | Type alias for `httprc.HTTPClient`; `*http.Client` satisfies it |
| `DefaultHTTPClient`, `WrapHTTPClientDefaults` | Helpers for building `*http.Client` values with the library's 30s timeout and redirect hardening |
| `WhitelistError` | Error sentinel returned by `Client.Fetch` when a URL is rejected by the whitelist |
| `HTTPStatusError`, `BodyTooLargeError`, `TransportError`, `ParseError` | Typed errors returned by `Client.Fetch` for non-whitelist failures — status, size cap, transport/IO, parse. Usable with `errors.Is` (zero-value) and `errors.As` for field extraction. |

### Option interfaces

| Interface | Where it's accepted | What it configures |
|-----------|--------------------|--------------------|
| `ClientOption` | `NewClient` | Client-specific or shared fetch policy |
| `CacheOption` | `NewCache` | Cache-specific or shared fetch policy |
| `GlobalFetchOption` | both `NewClient` and `NewCache` | Shared HTTP/parse policy (`WithHTTPClient`, `WithMaxBodySize`, `WithParseOptions`) |
| `RegisterOption` | `Cache.Register` (per URL) | Cache refresh interval, wait-ready |

### Safety defaults

- `NewClient()` with no `WithWhitelist` permits every URL
  (`InsecureWhitelist` semantics). This is the right default for
  compile-time-constant or trusted-config URLs: the trust decision
  is already made by the code that wrote the URL. For `jku`-style
  verification where the URL comes from an untrusted JWS header,
  the caller MUST pass `WithWhitelist` with a `MapWhitelist`,
  `RegexpWhitelist`, or custom `Whitelist` that restricts which
  destinations the Client will contact. jwx itself does not prepend
  a default deny for jku verification — the allowlist has to be
  configured on the fetcher explicitly.
- A restrictive `Whitelist` is applied to every redirect target,
  not just the initial URL. `Client.Fetch` wraps the HTTPClient's
  `CheckRedirect` so each hop of a 3xx chain is checked against the
  caller's `Whitelist`, preventing a hostile JWKS host from
  redirecting into an off-allowlist URL. This covers the common
  `*http.Client` case. If you supply a custom `HTTPClient`
  implementation via `WithHTTPClient`, you are responsible for
  policing redirect targets yourself.
- `Cache` is a cache, not a policy enforcer. It only ever contacts
  URLs you passed to `Register`. `Fetch` and `Lookup` return an
  error for URLs that haven't been registered — that's a cache
  miss, not a security check. There is no separate Whitelist
  concept on Cache because the set of URLs it will ever contact is
  already determined by your `Register` calls. Callers who need a
  whitelist check against dynamic URLs should use `Client` with
  `WithWhitelist`.
- `Cache` does NOT wrap its HTTPClient's `CheckRedirect`. A 302 from
  a registered host to an off-set URL is followed subject only to
  the underlying `*http.Client.CheckRedirect` (by default
  `DefaultHTTPClient`'s HTTPS-downgrade block + 5-hop cap, nothing
  else). Registration trusts the host, not the URL string — real
  deployments legitimately redirect a registered URL through a CDN
  URL, and a default-on "redirect must end in the registered set"
  check would either break those deployments or be trivially
  bypassable by an attacker who already controls the registered
  host. Operators who can't make the trust-the-host assumption
  must pin resolution at the Transport layer, serve the JWKS over
  a channel they trust end-to-end, or use `Client` with a
  restrictive `Whitelist`. Do not re-raise this as a security
  finding without first proposing a behavior change; the current
  behavior is documented, not accidental.
- `NewClient` / `NewCache` with no `WithHTTPClient` use
  `DefaultHTTPClient()`, which has a 30-second timeout, a 5-redirect
  cap, and a redirect policy that blocks HTTPS→HTTP scheme
  downgrades.
- `DefaultHTTPClient()` installs a dedicated `*http.Transport` rather
  than falling through to `http.DefaultTransport`. The dedicated
  transport has `Proxy: nil` (so `HTTP_PROXY` / `HTTPS_PROXY` are
  NOT honored — a JWKS fetcher silently following an env-var proxy
  is an SSRF pivot invisible to the caller) and pins
  `TLSClientConfig.MinVersion = tls.VersionTLS12`. Callers who need
  a proxy-aware transport (corporate egress) must construct their
  own `*http.Client` with `http.ProxyFromEnvironment` or a custom
  `Proxy` function and pass it via `WithHTTPClient`.
- Plaintext `http://` JWKS URLs are NOT rejected. The HTTPS-only
  redirect policy applies to redirect hops only; the scheme of the
  initial URL passed to `Client.Fetch` is the caller's
  responsibility. This is intentional and consistent with the
  trusted-URL default — scheme choice is delegated to the caller
  the same way host choice is. Callers who want plaintext refused
  should pre-validate or pass a `RegexpWhitelist` anchored on
  `^https://`. Do not re-raise this as a security finding without
  first proposing a behavior change; the current behavior is
  documented, not accidental.
- `Cache.Shutdown` is mandatory. `NewCache` calls `httprc.Client.Start`,
  which spawns background worker goroutines and refresh timers; those
  live until `Shutdown` is called. Callers MUST `defer cache.Shutdown(ctx)`
  right after `NewCache`. The godoc on `NewCache` and `Shutdown`, and
  the README Cache example, all state this. Do not re-flag it as a
  goroutine leak without first proposing a behavior change (e.g. a
  lifecycle tied to `ctx.Done()`); the current contract is documented.
- Error messages from `Client.Fetch`, `Cache.Fetch`, and the cache
  lookup/refresh family echo the caller-supplied URL verbatim so that
  operators can identify which JWKS endpoint failed. URLs containing
  userinfo credentials or sensitive query strings (access tokens, API
  keys) will therefore surface in returned errors and any configured
  `ErrorSink`. This is intentional — `jwkfetch` does not redact URLs,
  and callers that pass credential-bearing URLs are responsible for
  sanitizing them before passing. See the `jwkfetch` package doc for
  the full statement.

## Build / Test

Requires `GOEXPERIMENT=jsonv2` (jwx v4 dependency):

```
GOEXPERIMENT=jsonv2 go test ./...
```

## Files

| File | Purpose |
|------|---------|
| `jwkfetch.go` | Package doc, `Client` struct + `Fetch`, `HTTPClient`/`ErrorSink`/`TraceSink` aliases, `DefaultHTTPClient`, `WrapHTTPClientDefaults`, constants |
| `jwkfetch_test.go` | `Client` tests (whitelist defaults, allow/deny paths, transport errors) |
| `cache.go` | `Cache` struct + `NewCache` + `Register` + `Fetch` + `Lookup` family + internal `transformer` |
| `cache_test.go` | Cache refresh/backoff/concurrency tests |
| `cachedset.go` | `NewCachedSet` + internal `cachedSet` read-only `jwk.Set` wrapper |
| `whitelist.go` | `Whitelist`, `InsecureWhitelist`, `BlockAllWhitelist`, `MapWhitelist`, `RegexpWhitelist`, `WhitelistFunc`, `WhitelistError` sentinel |
| `options.go` | Option interface types and constructors (`WithHTTPClient`, `WithMaxBodySize`, `WithParseOptions`, `WithWhitelist`, `WithWaitReady`, `WithConstantInterval`, `WithMinInterval`, `WithMaxInterval`) |

## Branch Policy

| Branch | Purpose |
|--------|---------|
| `v*` (e.g. `v4`) | Release tags only. NEVER commit directly to these branches. |
| `develop/v*` (e.g. `develop/v4`) | Active development. All feature branches merge here. |
| Feature branches | Branch from `develop/v*`, merge back via PR. |

- Tags are cut from `v*` branches.
- `v*` branches should never be directly worked on.
- Regular development happens on `develop/v*` and feature branches.
