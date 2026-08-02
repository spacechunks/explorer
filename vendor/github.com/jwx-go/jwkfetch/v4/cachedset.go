package jwkfetch

import (
	"fmt"
	"iter"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v4/jwk"
)

// NewCachedSet creates a read-only jwk.Set that is a live view over
// an httprc-managed resource. Mutation methods (AddKey, Remove, Set,
// ...) return an error.
//
// The returned jwk.Set does NOT propagate caller context — the
// jwk.Set interface takes no context — so reads never block on
// resource readiness. Reads observe whatever snapshot the underlying
// httprc resource currently holds; background refreshes performed
// by the Cache controller are picked up transparently.
//
// If the underlying resource has not yet successfully fetched (for
// example, Register was called with WithWaitReady(false) while the
// first fetch is still in flight, or the resource is permanently
// failing), read methods return empty / not-found values:
//
//   - Len() returns 0.
//   - Key(i), LookupKeyID(kid) return (nil, false).
//   - Keys() returns nil.
//   - All(), Fields() yield nothing.
//   - Clone(), Field(), MarshalJSON() return an error.
//
// Callers that need a definitive "is this cache ready" signal
// should use Cache.Ready or Cache.Lookup with a context instead.
func NewCachedSet(r *httprc.ResourceBase[jwk.Set]) jwk.Set {
	return &cachedSet{r: r}
}

// cachedSet is a read-only jwk.Set backed by a cached resource.
type cachedSet struct {
	r *httprc.ResourceBase[jwk.Set]
}

// cached returns the current live snapshot held by the underlying
// httprc resource without blocking. If no successful fetch has
// landed yet, Resource() returns nil and we surface that as an
// error to the caller.
func (cs *cachedSet) cached() (jwk.Set, error) {
	set := cs.r.Resource()
	if set == nil {
		return nil, fmt.Errorf(`jwkfetch.CachedSet: underlying resource is not ready`)
	}
	return set, nil
}

func (*cachedSet) AddKey(_ jwk.Key) error {
	return fmt.Errorf(`jwkfetch.CachedSet is immutable`)
}

func (*cachedSet) Clear() error {
	return fmt.Errorf(`jwkfetch.CachedSet is immutable`)
}

func (*cachedSet) Set(_ string, _ any) error {
	return fmt.Errorf(`jwkfetch.CachedSet is immutable`)
}

func (*cachedSet) Remove(_ string) error {
	return fmt.Errorf(`jwkfetch.CachedSet is immutable`)
}

func (*cachedSet) RemoveKey(_ jwk.Key) error {
	return fmt.Errorf(`jwkfetch.CachedSet is immutable`)
}

func (cs *cachedSet) Clone() (jwk.Set, error) {
	set, err := cs.cached()
	if err != nil {
		return nil, err
	}
	return set.Clone()
}

func (cs *cachedSet) Field(name string) (any, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}
	return set.Field(name)
}

func (cs *cachedSet) Key(idx int) (jwk.Key, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}
	return set.Key(idx)
}

func (cs *cachedSet) Index(key jwk.Key) int {
	set, err := cs.cached()
	if err != nil {
		return -1
	}
	return set.Index(key)
}

func (cs *cachedSet) Keys() []string {
	set, err := cs.cached()
	if err != nil {
		return nil
	}
	return set.Keys()
}

func (cs *cachedSet) Len() int {
	set, err := cs.cached()
	if err != nil {
		return 0
	}
	return set.Len()
}

func (cs *cachedSet) LookupKeyID(kid string) (jwk.Key, bool) {
	set, err := cs.cached()
	if err != nil {
		return nil, false
	}
	return set.LookupKeyID(kid)
}

func (cs *cachedSet) All() iter.Seq2[int, jwk.Key] {
	set, err := cs.cached()
	if err != nil {
		return func(func(int, jwk.Key) bool) {}
	}
	return set.All()
}

func (cs *cachedSet) Fields() iter.Seq2[string, any] {
	set, err := cs.cached()
	if err != nil {
		return func(func(string, any) bool) {}
	}
	return set.Fields()
}

func (cs *cachedSet) MarshalJSON() ([]byte, error) {
	set, err := cs.cached()
	if err != nil {
		return nil, err
	}
	m, ok := set.(interface{ MarshalJSON() ([]byte, error) })
	if !ok {
		return nil, fmt.Errorf(`jwkfetch.CachedSet: underlying set does not implement MarshalJSON`)
	}
	return m.MarshalJSON()
}

func (cs *cachedSet) UnmarshalJSON(_ []byte) error {
	return fmt.Errorf(`jwkfetch.CachedSet is immutable`)
}
