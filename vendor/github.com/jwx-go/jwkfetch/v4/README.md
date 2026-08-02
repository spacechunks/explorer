# jwkfetch

HTTP-based JWK Set retrieval for
[github.com/lestrrat-go/jwx/v4](https://github.com/lestrrat-go/jwx).

This module provides two `jwk.Fetcher` implementations:

- **`Client`** — one-shot HTTPS fetches with whitelist, body-size cap,
  and parse-option control. Use it for ad-hoc retrieval or
  `jku`-style JWS verification.
- **`Cache`** — background-refreshed JWKS store backed by
  [httprc](https://github.com/lestrrat-go/httprc). Use it when you
  have a small, trusted set of JWKS URLs and want to amortize fetch
  cost across verifications.

It was extracted from the `jwk` package in jwx v4 so that the core
jwx module depends on neither `net/http` nor `httprc`.

## Install

```
go get github.com/jwx-go/jwkfetch/v4
```

Requires `GOEXPERIMENT=jsonv2`.

## Usage

### One-shot fetch with `Client`

A `Client` constructed with no options permits every URL. This is
the right default when the URL you are fetching is a compile-time
constant or comes from trusted configuration: you already made the
trust decision by writing the URL into your code, and a whitelist
would be redundant.

```go
client := jwkfetch.NewClient()
set, err := client.Fetch(ctx, "https://issuer.example/jwks.json")
```

When the URL originates from an untrusted source — most commonly
the `jku` header of a JWS handed to you by a peer — you MUST
restrict the reachable URLs with `WithWhitelist`. Without a
restrictive `Whitelist`, a hostile peer can point the fetcher at
any URL on any network the fetcher can reach (SSRF), or can hand
you a JWKS their own server controls and have their keys accepted
as "the issuer's keys":

```go
client := jwkfetch.NewClient(
    jwkfetch.WithWhitelist(
        jwkfetch.NewMapWhitelist().Add("https://issuer.example/jwks.json"),
    ),
)

// Wire into jws.WithVerifyAuto or jwt.WithVerifyAuto.
_, err := jws.Verify(signed, jws.WithVerifyAuto(client))
```

A restrictive `Whitelist` is applied to BOTH the initial URL and
every redirect target. If `issuer.example/jwks.json` responds with
`302 https://attacker.example/evil.json`, the redirect is rejected
even though the initial URL passed the whitelist — a hostile JWKS
host cannot bypass the allowlist by 302-ing into an off-list URL.

**Caveat**: the redirect-hop enforcement requires jwkfetch to wrap
the HTTP client's `CheckRedirect`, which only works when the
`HTTPClient` is a `*http.Client` (the overwhelmingly common case
including `DefaultHTTPClient()`). If you plug in a custom
`HTTPClient` implementation via `WithHTTPClient`, the redirect-hop
whitelist does NOT apply and you are on the hook for policing
redirects through your implementation's own mechanism.

**Plaintext HTTP is NOT rejected by default.** `DefaultHTTPClient()`
blocks HTTPS→HTTP downgrades on *redirect* hops, but does not
reject a plaintext `http://` URL passed directly to `Fetch`. This
is consistent with the trusted-URL default: the caller chose the
URL, and scheme is the caller's decision in the same way host is.
If you need to refuse plaintext, either pre-validate the URL
before calling `Fetch`, or pass a `RegexpWhitelist` whose patterns
are anchored with `^https://`:

```go
client := jwkfetch.NewClient(
    jwkfetch.WithWhitelist(
        jwkfetch.NewRegexpWhitelist().
            Add(regexp.MustCompile(`^https://issuer\.example/`)),
    ),
)
```

### Background-refreshed cache with `Cache`

```go
cache, err := jwkfetch.NewCache(ctx, httprc.NewClient())
if err != nil { ... }
defer cache.Shutdown(ctx)

err = cache.Register(ctx, "https://issuer.example/jwks.json",
    jwkfetch.WithMinInterval(5*time.Minute),
)

// Cache implements jwk.Fetcher:
_, err = jws.Verify(signed, jws.WithVerifyAuto(cache))
```

`Cache.Shutdown` is mandatory: `NewCache` spawns httprc background
workers and refresh timers that live until `Shutdown` is called. A
process that forgets to call it — or that replaces its cache on config
reload without shutting the old one down first — will leak a generation
of worker goroutines each time.

`Cache` has no whitelist of its own — it's a cache, so the set of
URLs it will ever contact is exactly the set you passed to
`Register`. `Fetch` and `Lookup` return an error for URLs that
haven't been registered. Use `Client` with `WithWhitelist` if you
need a whitelist check against a dynamic set of URLs.

## Options

Options that configure HTTP fetch policy work for both `NewClient` and
`NewCache`:

| Option | Description |
|--------|-------------|
| `WithHTTPClient(c)` | Override the `*http.Client` used for fetches (default: `DefaultHTTPClient()` — 30s timeout, 5-redirect cap, HTTPS→HTTP redirect-hop block, dedicated transport with no `HTTP_PROXY`/`HTTPS_PROXY` inheritance, TLS 1.2 floor; the initial URL scheme is not checked — plaintext `http://` is contacted as-is) |
| `WithMaxBodySize(n)` | Maximum response body bytes (default: 10 MB) |
| `WithParseOptions(...)` | `jwk.ParseOption` values passed through to `jwk.Parse` |

`Client`-only:

| Option | Description |
|--------|-------------|
| `WithWhitelist(w)` | URL allowlist consulted on the initial URL and every redirect hop (default: allow-all; set for any URL from an untrusted source) |

`Cache.Register` per-URL options:

| Option | Description |
|--------|-------------|
| `WithWaitReady(bool)` | Whether `Register` blocks until the first fetch completes (default: `true`) |
| `WithConstantInterval(d)` | Use a fixed refresh interval |
| `WithMinInterval(d)` | Minimum refresh interval |
| `WithMaxInterval(d)` | Maximum refresh interval |

## Whitelist types

`WithWhitelist` accepts any implementation of `jwkfetch.Whitelist`:

- `InsecureWhitelist{}` — allow every URL (the default when `WithWhitelist` isn't passed)
- `BlockAllWhitelist{}` — deny every URL (useful for tests, safety assertions, or intentionally-disabled code paths)
- `NewMapWhitelist().Add(url1).Add(url2)` — fixed allow-list of exact URLs
- `NewRegexpWhitelist().Add(pattern)` — pattern-based allow-list (patterns are **not** anchored for you; see [Origin-prefix or path-pattern match](#origin-prefix-or-path-pattern-match))
- `WhitelistFunc(func(string) bool)` — custom predicate

A restrictive `Whitelist` (anything other than `InsecureWhitelist{}`)
is checked on every URL the Client contacts, including the targets
of 3xx redirects.

**`Whitelist` applies to `Client` only.** `Cache` has no `Whitelist`
field and does not accept `WithWhitelist` — trying to pass it to
`NewCache` is a compile-time error. `Cache` is a cache, so the set
of URLs it will ever contact is exactly the set you passed to
`Register`; `Fetch` and `Lookup` return an error for anything else.
If you need a whitelist check against a set of URLs that isn't
known at `Register` time, use `Client` with `WithWhitelist` instead.

## Allowlist patterns: permit specific URLs, block everything else

The whitelist types above all have "fail closed" semantics: any URL
that doesn't match a listed entry / pattern / predicate is rejected
with a `WhitelistError`. There is no hidden catch-all that needs
disabling — if you construct a restrictive whitelist and it doesn't
match, the Client refuses the request. This applies identically to
the initial URL passed to `Fetch` and to every redirect target.

### Fixed list of exact URLs

Use `MapWhitelist` when the full set of URLs the Client will ever
contact is known at construction time. List every URL you need to
allow, including any URL you expect the server to redirect into:

```go
client := jwkfetch.NewClient(
    jwkfetch.WithWhitelist(
        jwkfetch.NewMapWhitelist().
            Add("https://issuer.example/.well-known/jwks.json").
            Add("https://issuer.example/keys/v2.json"), // known redirect target
    ),
)
```

If you list only the initial URL and the server 302s to a path you
didn't list, the redirect target will be rejected — which is
usually what you want, since a surprise 302 is how a hostile host
would attempt substitution.

### Origin-prefix or path-pattern match

When the issuer might redirect between paths on the same origin, or
when you want to allow any path under a known host, use
`RegexpWhitelist`:

```go
import "regexp"

client := jwkfetch.NewClient(
    jwkfetch.WithWhitelist(
        jwkfetch.NewRegexpWhitelist().
            Add(regexp.MustCompile(`^https://issuer\.example/`)),
    ),
)
```

Two things are load-bearing in that pattern:

1. The `^` anchor — without it, the regex matches if `issuer.example/`
   appears anywhere in the URL, e.g. `https://attacker.example/redirect?to=https://issuer.example/...`.
2. The literal `/` after the host — without it, the pattern matches
   `https://issuer.example.attacker.com/jwks.json`, because
   `issuer.example` is a prefix of `issuer.example.attacker.com`.
   Always put the path separator immediately after the hostname to
   stop greedy matching from escaping the host you meant.

### Custom policy

For anything more involved — a set known only at runtime, or a
policy that consults an external config — implement `Whitelist`
directly or wrap a closure with `WhitelistFunc`:

```go
allowed := loadTrustedJWKSURLsFromConfig() // map[string]struct{}
client := jwkfetch.NewClient(
    jwkfetch.WithWhitelist(jwkfetch.WhitelistFunc(func(u string) bool {
        _, ok := allowed[u]
        return ok
    })),
)
```

The function is called once for the initial URL and once for every
redirect target. Returning `false` rejects that hop with a
`WhitelistError`.

Errors returned when a URL is rejected match
`errors.Is(err, jwkfetch.WhitelistError())`.

## `Cache.CachedSet`

`Cache.CachedSet(url)` returns a read-only `jwk.Set` whose contents
always reflect the latest cached data. All mutation methods on the
returned set return errors.
