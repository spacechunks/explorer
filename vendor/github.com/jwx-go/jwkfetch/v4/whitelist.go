package jwkfetch

import (
	"errors"
	"fmt"
	"regexp"
)

var errDefaultWhitelistError = whitelistError{errors.New(`rejected by whitelist`)}

// Whitelist describes an interface for a URL whitelist that can be used
// to restrict URLs that jwkfetch.Client.Fetch can access.
type Whitelist interface {
	IsAllowed(string) bool
}

// WhitelistFunc is a function-based implementation of Whitelist.
type WhitelistFunc func(string) bool

func (f WhitelistFunc) IsAllowed(u string) bool {
	return f(u)
}

// InsecureWhitelist is a Whitelist implementation that allows every URL.
// It is the implicit default when a Client is constructed without a
// WithWhitelist option — the right choice for hard-coded or trusted-
// config JWKS URLs.
//
// Do NOT use InsecureWhitelist in any code path where the URL originates
// from untrusted input (for example, the `jku` header of a JWS). For
// those paths, construct a MapWhitelist, RegexpWhitelist, or custom
// Whitelist via WithWhitelist.
type InsecureWhitelist struct{}

func (InsecureWhitelist) IsAllowed(string) bool { return true }

// BlockAllWhitelist is a Whitelist that rejects every URL. Use it to
// construct a Client that explicitly refuses to fetch anything — for
// tests, safety assertions, or intentionally-disabled code paths.
type BlockAllWhitelist struct{}

func (BlockAllWhitelist) IsAllowed(string) bool { return false }

// MapWhitelist is a whitelist backed by a map of allowed URLs.
type MapWhitelist struct {
	urls map[string]struct{}
}

func NewMapWhitelist() MapWhitelist {
	return MapWhitelist{urls: make(map[string]struct{})}
}

func (wl MapWhitelist) Add(u string) MapWhitelist {
	wl.urls[u] = struct{}{}
	return wl
}

func (wl MapWhitelist) IsAllowed(u string) bool {
	_, ok := wl.urls[u]
	return ok
}

// RegexpWhitelist is a whitelist that uses regular expressions to match URLs.
//
// Patterns are NOT anchored for you. A URL is allowed if any pattern
// matches anywhere in it, so a naive pattern such as `issuer\.example`
// also matches hostile look-alikes like
// https://issuer.example.attacker.com/jwks.json or
// https://attacker.example/?x=issuer.example. Always anchor the start
// with ^, escape the literal dots with \., and terminate the host with
// a / (e.g. `^https://issuer\.example/`) so the pattern can only match
// the origin you intend. Prefer MapWhitelist when the set of URLs is
// known exactly; reach for RegexpWhitelist only when you genuinely need
// patterns.
type RegexpWhitelist struct {
	patterns []*regexp.Regexp
}

func NewRegexpWhitelist() *RegexpWhitelist {
	return &RegexpWhitelist{}
}

func (wl *RegexpWhitelist) Add(pat *regexp.Regexp) *RegexpWhitelist {
	wl.patterns = append(wl.patterns, pat)
	return wl
}

func (wl *RegexpWhitelist) IsAllowed(u string) bool {
	for _, pat := range wl.patterns {
		if pat.MatchString(u) {
			return true
		}
	}
	return false
}

type whitelistError struct {
	error
}

func (e whitelistError) Unwrap() error { return e.error }

func (whitelistError) Is(err error) bool {
	_, ok := err.(whitelistError)
	return ok
}

// WhitelistError returns an error sentinel that can be passed to
// errors.Is to check if an error was caused by a URL being rejected
// by a Whitelist.
func WhitelistError() error {
	return errDefaultWhitelistError
}

// HTTPStatusError is returned by Client.Fetch when the JWKS endpoint
// responds with a non-200 HTTP status. The StatusCode field carries
// the observed status so callers can branch retry/alert policy
// (e.g. retry on 5xx, alert on 4xx).
//
// Use errors.Is(err, HTTPStatusError{}) for a type check, or
// errors.As to extract the fields.
type HTTPStatusError struct {
	StatusCode int
	URL        string
}

func (e HTTPStatusError) Error() string {
	return fmt.Sprintf(`jwkfetch.Client.Fetch: request returned status %d, expected 200`, e.StatusCode)
}

func (HTTPStatusError) Is(err error) bool {
	_, ok := err.(HTTPStatusError)
	return ok
}

// BodyTooLargeError is returned when a JWKS response body exceeds
// the configured maximum (WithMaxBodySize, or the package default).
// The Limit field carries the cap in bytes.
//
// Returned by both Client.Fetch and the Cache refresh path (surfaced
// through the cache's configured ErrorSink). Use errors.Is(err,
// BodyTooLargeError{}) for a type check, or errors.As to extract
// the fields.
type BodyTooLargeError struct {
	Limit int64
	URL   string
}

func (e BodyTooLargeError) Error() string {
	return fmt.Sprintf(`jwkfetch: response body for %q exceeded max size of %d bytes`, e.URL, e.Limit)
}

func (BodyTooLargeError) Is(err error) bool {
	_, ok := err.(BodyTooLargeError)
	return ok
}

// TransportError wraps an HTTP transport-layer failure: request
// construction, http.Client.Do, or response body read. Unwrap returns
// the underlying stdlib error so callers can still reach net.Error,
// *url.Error, etc.
//
// Returned by Client.Fetch for all three Op values, and by the
// Cache refresh path for "read response body" only — the other Ops
// are reached only on the Client.Fetch code path. Use errors.Is(err,
// TransportError{}) for a type check, or errors.As to extract the
// fields.
// opReadResponseBody is the TransportError.Op value used when reading
// the HTTP response body fails.
const opReadResponseBody = "read response body"

type TransportError struct {
	URL string
	Op  string // "new request", "request", "read response body"
	Err error
}

func (e TransportError) Error() string {
	switch e.Op {
	case "new request":
		return fmt.Sprintf(`jwkfetch: failed to create new request: %s`, e.Err)
	case opReadResponseBody:
		return fmt.Sprintf(`jwkfetch: failed to read response body for %q: %s`, e.URL, e.Err)
	default:
		return fmt.Sprintf(`jwkfetch: request failed: %s`, e.Err)
	}
}

func (e TransportError) Unwrap() error { return e.Err }

func (TransportError) Is(err error) bool {
	_, ok := err.(TransportError)
	return ok
}

// ParseError wraps a jwk.Parse failure observed after the HTTP fetch
// has succeeded. Unwrap returns the underlying jwk parse error so
// existing errors.Is checks against jwk.ParseError() continue to work.
//
// Returned by both Client.Fetch and the Cache refresh path (surfaced
// through the cache's configured ErrorSink). Use errors.Is(err,
// jwkfetch.ParseError{}) for a type check, or errors.As to extract
// the fields.
type ParseError struct {
	URL string
	Err error
}

func (e ParseError) Error() string {
	return fmt.Sprintf(`jwkfetch: failed to parse JWK set from %q: %s`, e.URL, e.Err)
}

func (e ParseError) Unwrap() error { return e.Err }

func (ParseError) Is(err error) bool {
	_, ok := err.(ParseError)
	return ok
}
