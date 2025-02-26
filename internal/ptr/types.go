// Package ptr contains very small helper function to create pointers
// from specific non-pointer datatypes. This is to help with protobuf
// generated struct fields which require pointer values.
package ptr

func String(str string) *string {
	return &str
}

func Int32(i int32) *int32 {
	return &i
}

func UInt32(i uint32) *uint32 {
	return &i
}
