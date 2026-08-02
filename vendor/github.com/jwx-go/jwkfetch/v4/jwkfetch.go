// Package jwkfetch provides HTTP-based JWK Set retrieval for
// github.com/lestrrat-go/jwx/v4. It offers two complementary types,
// both of which implement jwk.Fetcher:
//
//   - Client: a one-shot HTTP JWKS fetcher with whitelist, body-size
//     cap, and per-fetch parse options. Use it for ad-hoc or jku-style
//     verification where the URL may be attacker-controllable.
//   - Cache: an httprc-backed store that keeps a fixed set of
//     registered JWKS URLs hot with background refresh. Use it when
//     you have a small, trusted list of JWKS endpoints and want to
//     amortize fetch cost across verifications.
//
// This package was extracted from the main jwk module so that the
// core jwx module does not depend on net/http or httprc.
//
// # URLs in error messages
//
// Error messages returned from Client.Fetch, Cache.Fetch, and related
// lookup/refresh methods include the URL that was passed to them so
// operators can identify which JWKS endpoint failed. This is
// intentional and is not redacted. URLs carrying userinfo
// (user:password@host) or sensitive query strings (for example
// ?access_token=... or ?api_key=...) will therefore appear in
// returned errors, any wrapping application logs, and the httprc
// ErrorSink configured on a Cache. Callers that fetch JWKS from
// credential-bearing URLs are responsible for sanitizing those URLs
// before passing them to jwkfetch, or for accepting that the
// credentials may surface in error output.
package jwkfetch

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/option/v3"
)

// HTTPClient is the minimal HTTP client interface used by both Client
// and the httprc-backed Cache. It is a type alias for
// httprc.HTTPClient so either type accepts the same values; *http.Client
// satisfies it.
type HTTPClient = httprc.HTTPClient

// ErrorSink is the httprc error sink interface, re-exported for
// convenience when configuring an httprc.Client for use with Cache.
type ErrorSink = httprc.ErrorSink

// TraceSink is the httprc trace sink interface, re-exported for
// convenience when configuring an httprc.Client for use with Cache.
type TraceSink = httprc.TraceSink

// defaultClientMaxBodySize is the default maximum number of bytes read
// from an HTTP response body when Client.MaxBodySize is not set. The
// same default is used by Cache's internal Transformer when
// Cache.Client.MaxBodySize is zero.
const defaultClientMaxBodySize int64 = 10 * 1024 * 1024

// defaultFetchTimeout is the default timeout applied to the HTTP
// client constructed by DefaultHTTPClient. It prevents malicious or
// unresponsive JWKS endpoints from hanging indefinitely (e.g.
// slowloris-style DoS).
const defaultFetchTimeout = 30 * time.Second

// defaultMaxRedirects is the maximum number of HTTP redirects the
// default fetch client will follow. Intentionally lower than Go's
// default of 10 to limit redirect chain abuse.
const defaultMaxRedirects = 5

// Client is a one-shot JWKS fetcher. It implements jwk.Fetcher.
//
// A Client is constructed via NewClient with functional options. Its
// internal state is immutable after construction and safe to share
// across goroutines.
//
// A Client constructed with no WithWhitelist permits every URL,
// matching the common case of fetching a hard-coded or trusted-config
// JWKS endpoint. For `jku`-style verification where the URL comes
// from an untrusted JWS header, the caller MUST restrict reachable
// URLs via WithWhitelist — see WithWhitelist for the available
// Whitelist types.
//
// Fetch does not reject plaintext `http://` URLs. The trusted-URL
// default delegates scheme choice to the caller, the same way it
// delegates host choice: if you hand the Client an `http://` URL,
// it will contact it in cleartext. Callers who need to refuse
// plaintext must either pre-validate the URL before calling Fetch,
// or pass a Whitelist whose patterns are anchored with `^https://`.
// The default redirect policy blocks HTTPS→HTTP downgrades on
// redirect hops, but does not retroactively reject a plaintext
// initial URL.
//
// When a restrictive Whitelist is configured, it is applied to BOTH
// the initial URL and every redirect target. The Client achieves
// this by wrapping the HTTPClient's CheckRedirect with a Whitelist
// pre-check, so a hostile JWKS host cannot 302 into an off-allowlist
// URL. This wrapping only applies when HTTPClient is a *http.Client
// (the overwhelmingly common case); if you supply a custom
// HTTPClient implementation via WithHTTPClient, you are responsible
// for applying the Whitelist to redirect targets yourself.
type Client struct {
	httpClient   HTTPClient
	whitelist    Whitelist
	maxBodySize  int64
	parseOptions []jwk.ParseOption
	// initErr captures construction-time misconfiguration that the
	// current NewClient signature cannot return. Fetch checks this
	// first and returns the error on every call. A future release
	// may promote NewClient to return (*Client, error); until then,
	// init errors surface from Fetch.
	initErr error
}

// NewClient constructs a one-shot JWKS fetcher. The returned Client
// implements jwk.Fetcher.
//
// With no WithWhitelist option, the Client permits every URL. This is
// the right default for trusted, hard-coded JWKS URLs. For `jku`
// verification or any fetch whose URL comes from an attacker-
// controllable source, pass WithWhitelist with a MapWhitelist,
// RegexpWhitelist, or other Whitelist implementation that restricts
// the reachable URLs.
//
// WithWhitelist(nil) is a configuration bug — nil cannot be a
// meaningful policy, and silently treating it as allow-all would
// turn a hardened deployment into an SSRF tool. NewClient captures
// the error and returns it from every Fetch call. The returned
// Client is non-nil so callers can defer through their own setup;
// every Fetch will fail until the construction is corrected.
//
// FUTURE COMPATIBILITY: a later release may promote NewClient's
// signature to (*Client, error) so construction-time errors surface
// at construction time rather than first Fetch. Code written today
// can remain compatible by checking the first Fetch error like any
// other failure path.
func NewClient(options ...ClientOption) *Client {
	c := &Client{}
	for _, opt := range options {
		switch opt.Ident() {
		case identHTTPClient{}:
			c.httpClient = option.MustGet[HTTPClient](opt)
		case identWhitelist{}:
			// Use Get rather than MustGet so a nil Whitelist (a typed
			// nil interface stored in the option, or any other value
			// that fails the type assertion) surfaces as our own
			// error rather than a panic deep inside the option pkg.
			w, _ := option.Get[Whitelist](opt)
			if w == nil {
				c.initErr = fmt.Errorf(`jwkfetch.NewClient: WithWhitelist requires a non-nil Whitelist; pass InsecureWhitelist{} for explicit allow-all or BlockAllWhitelist{} for deny-all`)
				continue
			}
			c.whitelist = w
		case identMaxBodySize{}:
			c.maxBodySize = option.MustGet[int64](opt)
		case identParseOptions{}:
			c.parseOptions = option.MustGet[[]jwk.ParseOption](opt)
		}
	}

	// Default: no WithWhitelist option means "permit every URL".
	// Storing the concrete type removes the nil-check from Fetch's
	// hot path and lets the redirect-hop wrapping below decide once
	// whether the Whitelist is restrictive.
	if c.whitelist == nil {
		c.whitelist = InsecureWhitelist{}
	}

	// Resolve the HTTPClient now. If the caller did not pass one, we
	// materialize the default here (rather than lazy-initializing
	// inside Fetch) so that any whitelist-driven CheckRedirect
	// wrapping can be applied exactly once at construction time.
	//
	// A caller-supplied *http.Client is used as-is. The library does
	// NOT silently impose its redirect cap or HTTPS-downgrade block
	// on it — that's the caller's explicit choice. Callers who want
	// the defaults applied to their own client should compose:
	//
	//	WithHTTPClient(jwkfetch.WrapHTTPClientDefaults(myClient))
	//
	// See WithHTTPClient's godoc for the full trade-off.
	if c.httpClient == nil {
		c.httpClient = DefaultHTTPClient()
	}

	// When the Whitelist is restrictive (anything other than
	// InsecureWhitelist{}), interpose it on redirect targets too by
	// wrapping the HTTPClient's CheckRedirect. A hostile JWKS host
	// must not be able to 302 into an off-allowlist URL. We only
	// wrap *http.Client values; exotic HTTPClient implementations
	// are left untouched, and the caller is responsible for
	// policing redirects through their custom implementation.
	//
	// Order: whitelist → defaultCheckRedirect → orig. The default
	// policy (5-redirect cap, HTTPS-downgrade block) MUST run even
	// when the caller's *http.Client brought its own CheckRedirect
	// — otherwise a permissive caller policy (or one that ignores
	// the cap) silently disables those library-level protections.
	// This mirrors WrapHTTPClientDefaults's chaining; the two
	// helpers should keep this order in sync.
	if _, insecure := c.whitelist.(InsecureWhitelist); !insecure {
		if hc, ok := c.httpClient.(*http.Client); ok {
			cloned := *hc
			orig := cloned.CheckRedirect
			wl := c.whitelist
			cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				if !wl.IsAllowed(req.URL.String()) {
					return whitelistError{fmt.Errorf(`jwkfetch.Client.Fetch: redirect target %q has been rejected by whitelist`, req.URL.String())}
				}
				if err := defaultCheckRedirect(req, via); err != nil {
					return err
				}
				if orig != nil {
					return orig(req, via)
				}
				return nil
			}
			c.httpClient = &cloned
		}
	}

	return c
}

// Fetch retrieves a JWK Set from the given URL. It implements
// jwk.Fetcher.
//
// If the Client was constructed with a restrictive Whitelist
// (anything other than InsecureWhitelist{}), that Whitelist is
// consulted against the initial URL and every redirect target —
// ensuring a hostile JWKS host cannot 302 into an off-allowlist URL.
// See the Client doc for the exact semantics and the non-*http.Client
// caveat.
//
// A Client constructed without WithWhitelist permits every URL.
//
// Error messages from Fetch — whitelist rejections, status mismatches,
// body-size overruns, parse failures, and transport errors — include
// u verbatim so callers can identify which URL failed. If u contains
// credentials in userinfo or query strings, those will appear in the
// returned error and any log or sink that records it; sanitize u at
// the caller if that is a concern. See the package doc for the full
// statement.
func (c *Client) Fetch(ctx context.Context, u string) (jwk.Set, error) {
	// Surface any construction-time misconfiguration captured in
	// NewClient. This is the sticky-error mechanism that lets
	// NewClient report errors without yet committing to a
	// (*Client, error) signature — see NewClient's "FUTURE
	// COMPATIBILITY" note.
	if c.initErr != nil {
		return nil, c.initErr
	}
	if !c.whitelist.IsAllowed(u) {
		return nil, whitelistError{fmt.Errorf(`jwkfetch.Client.Fetch: url %q has been rejected by whitelist`, u)}
	}

	maxBodySize := c.maxBodySize
	if maxBodySize <= 0 {
		maxBodySize = defaultClientMaxBodySize
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, TransportError{URL: u, Op: "new request", Err: err}
	}

	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, TransportError{URL: u, Op: "request", Err: err}
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return nil, HTTPStatusError{StatusCode: res.StatusCode, URL: u}
	}

	// LimitReader caps memory at maxBodySize+1; reading +1 byte lets us detect
	// oversized responses. We intentionally skip a Content-Length pre-check because
	// the header is untrustworthy (server-controlled, absent in chunked transfers).
	// Slow-trickle attacks are mitigated by context deadlines and http.Client.Timeout.
	buf, err := io.ReadAll(io.LimitReader(res.Body, maxBodySize+1))
	if err != nil {
		return nil, TransportError{URL: u, Op: opReadResponseBody, Err: err}
	}
	if int64(len(buf)) > maxBodySize {
		return nil, BodyTooLargeError{Limit: maxBodySize, URL: u}
	}

	set, err := jwk.Parse(buf, c.parseOptions...)
	if err != nil {
		return nil, ParseError{URL: u, Err: err}
	}
	return set, nil
}

// DefaultHTTPClient returns a new *http.Client configured with the
// defaults used by jwkfetch: a 30-second overall request timeout, a
// 5-redirect cap, a redirect policy that blocks HTTPS-to-HTTP scheme
// downgrades, and a dedicated *http.Transport that does NOT honor
// HTTP_PROXY / HTTPS_PROXY environment variables and pins the TLS
// floor at 1.2.
//
// The returned *http.Client does not share connection pools, TLS
// session state, or proxy configuration with the process-global
// http.DefaultTransport. This is deliberate: a JWKS fetcher that
// silently follows an attacker-influenced proxy env var is an SSRF
// pivot invisible to the caller. Callers who need a proxy-aware
// transport (corporate egress, MITM-inspecting enterprise proxy)
// must construct their own *http.Client with http.ProxyFromEnvironment
// or a custom Proxy function and pass it via WithHTTPClient.
//
// The redirect policy only applies to subsequent hops: the scheme of
// the INITIAL URL passed to Client.Fetch is NOT checked, and a
// plaintext `http://` JWKS URL will be contacted in cleartext. See
// the Client doc for the rationale and for how to opt into a
// plaintext rejection via Whitelist patterns.
//
// Useful for callers who want to start from the library's default
// protections and wrap them (e.g. adding a custom TLS configuration).
func DefaultHTTPClient() *http.Client {
	return WrapHTTPClientDefaults(&http.Client{
		Transport: newDefaultTransport(),
	})
}

// newDefaultTransport builds the *http.Transport used by
// DefaultHTTPClient. It is intentionally separate from
// http.DefaultTransport so that jwkfetch's defaults do not inherit
// process-global proxy env vars, connection pools, or TLS session
// state. See DefaultHTTPClient for rationale.
func newDefaultTransport() *http.Transport {
	return &http.Transport{
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   2,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

// WrapHTTPClientDefaults returns a shallow copy of the given
// *http.Client with the library's default safety behaviors applied.
// Existing client settings (Transport, Jar, etc.) are preserved.
//
//   - Timeout: applied only when the client has no timeout set
//     (zero value).
//   - CheckRedirect: if the client already has one, the library's
//     redirect policy runs first; if it passes, the original
//     CheckRedirect is called. If the client has no CheckRedirect,
//     the library's policy is used directly.
//
// Useful when callers need to bring their own *http.Client (e.g. for
// custom TLS configuration) but still want the library's redirect
// hardening.
func WrapHTTPClientDefaults(client *http.Client) *http.Client {
	cloned := *client
	if cloned.Timeout == 0 {
		cloned.Timeout = defaultFetchTimeout
	}
	orig := cloned.CheckRedirect
	if orig == nil {
		cloned.CheckRedirect = defaultCheckRedirect
	} else {
		cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if err := defaultCheckRedirect(req, via); err != nil {
				return err
			}
			return orig(req, via)
		}
	}
	return &cloned
}

// defaultCheckRedirect is the CheckRedirect policy for the default
// HTTP client. It prevents HTTPS-to-HTTP scheme downgrades and caps
// the total number of redirects.
//
// The HTTPS-only enforcement applies to redirect hops only — the
// scheme of the initial URL passed to Client.Fetch is the caller's
// responsibility. A plaintext `http://` URL passed directly to Fetch
// is contacted in cleartext.
//
// This does NOT protect against redirects to private/internal IP
// addresses. For full SSRF protection, callers should provide a custom
// http.Client via Client.HTTPClient whose Transport.DialContext
// validates destination IPs.
func defaultCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= defaultMaxRedirects {
		return fmt.Errorf(`jwkfetch: stopped after %d redirects`, defaultMaxRedirects)
	}

	// Prevent HTTPS → HTTP scheme downgrade at any hop.
	// via[len(via)-1] is the immediately previous request in the chain.
	if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
		return fmt.Errorf(`jwkfetch: redirect from HTTPS to non-HTTPS URL %q is not allowed`, req.URL.Redacted())
	}
	return nil
}
