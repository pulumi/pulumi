// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package rt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStable ensures that property enumeration order is stable from one run to the next.
func TestStable(t *testing.T) {
	t.Parallel()

	props := NewPropertyMap()

	// Seed the map.
	ks := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, k := range ks {
		props.Set(PropertyKey(string(k)), Null)
	}

	// Observe an initial key ordering.
	observed := props.Stable()

	// Finally validate that the key ordering never changes (well, okay, check 100 iterations).
	for i := 0; i < 100; i++ {
		for j, k := range props.Stable() {
			assert.Equal(t, observed[j], k)
		}
	}
}

// TestChrono ensures that property enumeration order returns keys in chronological order.  Note that this will need to
// change once we adopt ECMAScript's ordering (https://tc39.github.io/ecma262/#sec-ordinaryownpropertykeys).
func TestChrono(t *testing.T) {
	t.Parallel()

	props := NewPropertyMap()

	// Just add keys to the map and then ensure it enumerates chronologically.
	ks := "abcdefghijklmnopqrstuvwxyz0123456789"
	for _, k := range ks {
		props.Set(PropertyKey(string(k)), Null)
	}
	for i, k := range props.Stable() {
		assert.Equal(t, string(ks[i]), string(k))
	}
}
