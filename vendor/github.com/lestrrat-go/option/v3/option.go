package option

import "fmt"

// Interface defines the minimum interface that an option must fulfill.
// It uses an unexported method to restrict implementations to this package
// or types that embed Interface.
type Interface interface {
	// Ident returns the "identity" of this option, a unique identifier that
	// can be used to differentiate between options
	Ident() any

	value() any
}

// Option is a typed option that stores a value of type T along with
// an identifier. It implements Interface for heterogeneous storage
// and provides a typed Value() method for direct access.
type Option[T any] struct {
	ident any
	val   T
}

// New creates a new Option with the given identity and value.
// It returns a concrete *Option[T] so callers can use the typed Value() method.
// *Option[T] also satisfies Interface for heterogeneous storage.
func New[T any](ident any, v T) *Option[T] {
	return &Option[T]{
		ident: ident,
		val:   v,
	}
}

func (p *Option[T]) Ident() any {
	return p.ident
}

// Value returns the stored value with its original type.
func (p *Option[T]) Value() T {
	return p.val
}

func (p *Option[T]) value() any {
	return p.val
}

func (p *Option[T]) String() string {
	return fmt.Sprintf(`%v(%v)`, p.ident, p.val)
}

// Get extracts the value from an Interface with type safety.
// Returns the zero value and false if the stored value is not of type T.
func Get[T any](opt Interface) (T, bool) {
	v, ok := opt.value().(T)
	return v, ok
}

// MustGet extracts the value from an Interface, panicking if the
// stored value is not of type T. Use this inside switch cases on
// Ident() where the type is guaranteed.
func MustGet[T any](opt Interface) T {
	v, ok := opt.value().(T)
	if !ok {
		var zero T
		panic(fmt.Sprintf("option: expected %T, got %T", zero, opt.value()))
	}
	return v
}
