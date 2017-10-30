// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	cryptorand "crypto/rand"
	"crypto/sha1"
	"encoding/hex"

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

// NewUniqueHex generates a new "random" hex string for use by resource providers.  It has the given
// optional prefix and the total length is capped to the maxlen.
//
// Notes:
//  1. capping to maxlen necessarily increases the risk of collisions.
//  2. If there isn't enough room for the prefix, it will be dropped to ensure an reasonably unique
//     hex.
//  3. When there isn't enough room for the prefix and the amount of randomness asked for,
//     an attempt will be kept to preserve the prefix.  However, if the randomness drops too
//     low, then the prefix will be dropped to ensure uniqueness.
func NewUniqueHex(prefix string, maxlen, randlen int) string {
	if randlen == -1 {
		randlen = sha1.Size // default to SHA1 size.
	}

	bs := make([]byte, randlen)
	n, err := cryptorand.Read(bs)
	contract.Assert(err == nil)
	contract.Assert(n == len(bs))

	str := prefix + hex.EncodeToString(bs)
	strLen := len(str)

	if maxlen != -1 && strLen > maxlen {
		// Our string is longer than the length requested.  We can't just truncate from the left, as
		// there may not be enough randomness in the string (due to the fixed prefix).  If we can
		// get at least 8 characters of randomness, then attempt to keep the prefix in.  Otherwise,
		// we just take from the right side of the string to keep as much randomness as possible.
		if maxlen-len(prefix) >= 8 {
			return str[:maxlen]
		}

		// The string we've generated is larger than the requested string.  Ensure the least change
		// of collisions by extracting from the right side of the string (the part with the most
		// randomness).
		return str[strLen-maxlen:]
	}

	return str
}

// NewUniqueHexID generates a new "random" hex ID for use by resource providers.  It has the given
// optional prefix and the total length is capped to the maxlen.  Note that capping to maxlen
// necessarily increases the risk of collisions.
func NewUniqueHexID(prefix string, maxlen, randlen int) ID {
	return ID(NewUniqueHex(prefix, maxlen, randlen))
}
