// Package ptr contains a very small helper function to create pointers
// from specific non-pointer datatypes. This is to help with protobuf
// generated struct fields which require pointer values.
package ptr

func Pointer[T any](v T) *T {
	return &v
}
