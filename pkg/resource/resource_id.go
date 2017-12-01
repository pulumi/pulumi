// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/pulumi/pulumi/pkg/util/contract"
)

// ID is a unique resource identifier; it is managed by the provider and is mostly opaque to Lumi.
type ID string

// String converts a resource ID into a string.
func (id ID) String() string {
	return string(id)
}

// StringPtr converts an optional ID into an optional string.
func (id *ID) StringPtr() *string {
	if id == nil {
		return nil
	}
	ids := (*id).String()
	return &ids
}

// IDStrings turns an array of resource IDs into an array of strings.
func IDStrings(ids []ID) []string {
	ss := make([]string, len(ids))
	for i, id := range ids {
		ss[i] = id.String()
	}
	return ss
}

// MaybeID turns an optional string into an optional resource ID.
func MaybeID(s *string) *ID {
	var ret *ID
	if s != nil {
		id := ID(*s)
		ret = &id
	}
	return ret
}

// NewUniqueHex generates a new "random" hex string for use by resource providers. It will take
// the prefix and add 8 random chars to it.  If this is greater in length than maxlen an error
// will be returned.
func NewUniqueHex(prefix string, maxlen int) (string, error) {
	const randChars = 8

	if maxlen != -1 {
		// Each byte of randomness will create two hex chars.
		if len(prefix)+randChars > maxlen {
			return "", fmt.Errorf("Name '%s' is longer than maximum length %v", prefix, maxlen-randChars)
		}
	}

	bs := make([]byte, randChars/2)
	n, err := cryptorand.Read(bs)
	contract.Assert(err == nil)
	contract.Assert(n == len(bs))

	str := prefix + hex.EncodeToString(bs)
	return str, nil
}

// NewUniqueHexID generates a new "random" hex ID for use by resource providers.  It has the given
// optional prefix and the total length is capped to the maxlen.  Note that capping to maxlen
// necessarily increases the risk of collisions.
func NewUniqueHexID(prefix string, maxlen int) (ID, error) {
	u, err := NewUniqueHex(prefix, maxlen)
	if err != nil {
		return "", err
	}

	return ID(u), nil
}
