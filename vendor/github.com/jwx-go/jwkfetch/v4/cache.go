package jwkfetch

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/option/v3"
)

// Cache is an httprc-backed container that keeps JWK Sets hot for a
// fixed set of registered URLs. It implements jwk.Fetcher by serving
// the cached Set on Fetch and erroring on unregistered URLs.
//
// A Cache is constructed via NewCache. The HTTP transport, body-size
// cap, and parse options supplied to NewCache apply uniformly to
// every registered URL. Per-URL knobs (refresh interval, wait-ready)
// are passed to Register as RegisterOption values.
//
// Cache has no Whitelist concept of its own because the set of URLs
// it will ever contact is exactly the set passed to Register. Fetch
// and Lookup return an error for URLs that haven't been registered —
// that's a cache miss, not a security check. Callers who need a
// whitelist check against dynamic URLs should use Client with
// WithWhitelist via jwk.Fetcher instead.
//
// Unlike Client, Cache does NOT wrap its HTTPClient's CheckRedirect.
// Registration trusts the host, not just the URL string: redirect
// targets encountered while fetching a registered URL are not
// re-checked against the registration set. Concretely, if you
// register https://issuer.example/jwks.json and that host returns
// a 302 to https://attacker.example/jwks.json, the cache will
// follow the redirect subject only to the HTTPClient's own
// CheckRedirect policy — by default, that's DefaultHTTPClient's
// HTTPS-downgrade block and 5-hop cap, and nothing else. If a
// registered JWKS endpoint's DNS, CDN, or origin could be
// compromised into issuing such redirects, either pin resolution
// at the Transport layer, serve the JWKS over a channel whose
// endpoint you trust end-to-end, or use Client with a restrictive
// Whitelist instead.
//
// Error messages produced by Cache — including those surfaced
// through the httprc.ErrorSink configured on its underlying client
// — include registered URLs verbatim. Sanitize URLs before passing
// them to Register, Fetch, Lookup, Refresh, or Unregister if they
// contain credentials in userinfo or query strings that must not
// appear in logs. See the package doc for the full statement.
type Cache struct {
	httpClient   HTTPClient
	maxBodySize  int64
	parseOptions []jwk.ParseOption
	ctrl         httprc.Controller
}

// transformer converts an HTTP response to a jwk.Set. It is used
// internally by Cache.Register and is not part of the public API.
type transformer struct {
	parseOptions []jwk.ParseOption
	maxBodySize  int64
}

func (t transformer) Transform(_ context.Context, res *http.Response) (jwk.Set, error) {
	maxBody := t.maxBodySize
	if maxBody <= 0 {
		maxBody = defaultClientMaxBodySize
	}

	u := res.Request.URL.String()

	buf, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if err != nil {
		return nil, TransportError{URL: u, Op: opReadResponseBody, Err: err}
	}
	if int64(len(buf)) > maxBody {
		return nil, BodyTooLargeError{Limit: maxBody, URL: u}
	}

	set, err := jwk.Parse(buf, t.parseOptions...)
	if err != nil {
		return nil, ParseError{URL: u, Err: err}
	}

	return set, nil
}

// NewCache creates a new Cache backed by the given httprc client.
// Fetch policy options (WithHTTPClient, WithMaxBodySize,
// WithParseOptions) apply uniformly to every URL registered on the
// Cache.
//
// NewCache starts the underlying httprc.Client, which spawns background
// worker goroutines. Callers MUST call Cache.Shutdown when the Cache is
// no longer needed to release those workers; see Shutdown for details.
func NewCache(ctx context.Context, client *httprc.Client, options ...CacheOption) (*Cache, error) {
	c := &Cache{}
	for _, opt := range options {
		switch opt.Ident() {
		case identHTTPClient{}:
			c.httpClient = option.MustGet[HTTPClient](opt)
		case identMaxBodySize{}:
			c.maxBodySize = option.MustGet[int64](opt)
		case identParseOptions{}:
			c.parseOptions = option.MustGet[[]jwk.ParseOption](opt)
		}
	}

	ctrl, err := client.Start(ctx)
	if err != nil {
		return nil, fmt.Errorf(`failed to start httprc.Client: %w`, err)
	}
	c.ctrl = ctrl
	return c, nil
}

// Register registers a URL to be managed by the cache. HTTP transport,
// body-size cap, and parse options are inherited from the NewCache
// options; RegisterOption values only control cache-specific per-URL
// knobs (refresh interval, wait-ready).
func (c *Cache) Register(ctx context.Context, u string, options ...RegisterOption) error {
	var resourceOptions []httprc.NewResourceOption
	waitReady := true
	for _, opt := range options {
		switch opt.Ident() {
		case identWaitReady{}:
			waitReady = option.MustGet[bool](opt)
		case identResourceOption{}:
			resourceOptions = append(resourceOptions, option.MustGet[httprc.NewResourceOption](opt))
		}
	}

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = DefaultHTTPClient()
	}
	resourceOptions = append(resourceOptions, httprc.WithHTTPClient(httpClient))

	r, err := httprc.NewResource[jwk.Set](u, &transformer{
		parseOptions: c.parseOptions,
		maxBodySize:  c.maxBodySize,
	}, resourceOptions...)
	if err != nil {
		return fmt.Errorf(`failed to create httprc.Resource: %w`, err)
	}
	if err := c.ctrl.Add(ctx, r, httprc.WithWaitReady(waitReady)); err != nil {
		return fmt.Errorf(`failed to add resource: %w`, err)
	}
	return nil
}

// Fetch implements jwk.Fetcher. It returns the cached jwk.Set for u,
// or an error if u has not been registered. Register the URL via
// Cache.Register before calling Fetch.
//
// As with Client.Fetch, error messages include u verbatim so callers
// can identify which URL failed; see the Cache and package docs for
// caller-responsibility details on URL sanitization.
func (c *Cache) Fetch(ctx context.Context, u string) (jwk.Set, error) {
	if !c.IsRegistered(ctx, u) {
		return nil, fmt.Errorf(`jwkfetch.Cache: url %q is not registered`, u)
	}
	return c.Lookup(ctx, u)
}

// LookupResource returns the underlying httprc resource for the given URL.
func (c *Cache) LookupResource(ctx context.Context, u string) (*httprc.ResourceBase[jwk.Set], error) {
	r, err := c.ctrl.Lookup(ctx, u)
	if err != nil {
		return nil, fmt.Errorf(`failed to lookup resource %q: %w`, u, err)
	}
	//nolint:forcetypeassert
	return r.(*httprc.ResourceBase[jwk.Set]), nil
}

// Lookup retrieves the cached jwk.Set for the given URL.
func (c *Cache) Lookup(ctx context.Context, u string) (jwk.Set, error) {
	r, err := c.LookupResource(ctx, u)
	if err != nil {
		return nil, fmt.Errorf(`failed to lookup resource %q: %w`, u, err)
	}
	set := r.Resource()
	if set == nil {
		return nil, fmt.Errorf(`resource %q is not ready`, u)
	}
	return set, nil
}

// Ready returns true if the given URL's resource is ready.
func (c *Cache) Ready(ctx context.Context, u string) bool {
	r, err := c.LookupResource(ctx, u)
	if err != nil {
		return false
	}
	return r.Ready(ctx) == nil
}

// Refresh re-fetches the resource and updates the cache.
func (c *Cache) Refresh(ctx context.Context, u string) (jwk.Set, error) {
	if err := c.ctrl.Refresh(ctx, u); err != nil {
		return nil, fmt.Errorf(`failed to refresh resource %q: %w`, u, err)
	}
	return c.Lookup(ctx, u)
}

// IsRegistered returns true if the URL has been registered.
func (c *Cache) IsRegistered(ctx context.Context, u string) bool {
	_, err := c.LookupResource(ctx, u)
	return err == nil
}

// Unregister removes the URL from the cache.
func (c *Cache) Unregister(ctx context.Context, u string) error {
	return c.ctrl.Remove(ctx, u)
}

// Shutdown stops the cache controller and releases the httprc worker
// goroutines that NewCache started. Callers MUST call Shutdown when the
// Cache is no longer needed; otherwise httprc workers and background
// refresh timers leak for the lifetime of the process. A process that
// repeatedly replaces its Cache (for example on config reload) without
// shutting the previous one down will stack another generation of
// workers on every reload.
//
// The expected pattern is:
//
//	cache, err := jwkfetch.NewCache(ctx, httprc.NewClient())
//	if err != nil { ... }
//	defer cache.Shutdown(ctx)
func (c *Cache) Shutdown(ctx context.Context) error {
	return c.ctrl.ShutdownContext(ctx)
}

// CachedSet returns a jwk.Set backed by the cache. All mutation
// operations on the returned set return errors.
func (c *Cache) CachedSet(u string) (jwk.Set, error) {
	r, err := c.LookupResource(context.Background(), u)
	if err != nil {
		return nil, fmt.Errorf(`failed to lookup resource %q: %w`, u, err)
	}
	return NewCachedSet(r), nil
}
