//go:build !go1.20

package httpstate

import (
	"encoding/json"
	"unsafe"
)

// unsafeAsString converts the contents of the given json.RawMessage into a string without allocating. The contents
// of the returned string will change if the json.RawMessage is mutated before the referenced string goes out of scope.
// It is imperative that callers ensure that the lifetime of the return value does not exceed the lifetime of the
// input contents.
func unsafeAsString(raw json.RawMessage) string {
	// NOTE: this depends on the fact that the layout of a byte slice and a string share a common header. Both of these
	// types are laid out as (ptr, len).
	return *(*string)(unsafe.Pointer(&raw))
}
