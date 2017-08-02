// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package resource

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"

	"github.com/pulumi/pulumi-fabric/pkg/eval/rt"
	"github.com/pulumi/pulumi-fabric/pkg/util/contract"
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

const (
	// IDProperty is the special ID property name.
	IDProperty = rt.PropertyKey("id")
	// IDPropertyKey is the special ID property name for resource maps.
	IDPropertyKey = PropertyKey("id")
)

// NewUniqueHex generates a new "random" hex string for use by resource providers.  It has the given optional prefix and
// the total length is capped to the maxlen.  Note that capping to maxlen necessarily increases the risk of collisions.
func NewUniqueHex(prefix string, maxlen, randlen int) string {
	if randlen == -1 {
		randlen = sha1.Size // default to SHA1 size.
	}

	bs := make([]byte, randlen)
	n, err := rand.Read(bs)
	contract.Assert(err == nil)
	contract.Assert(n == len(bs))

	str := prefix + hex.EncodeToString(bs)
	if maxlen != -1 && len(str) > maxlen {
		str = str[:maxlen]
	}
	return str
}

// NewUniqueHexID generates a new "random" hex ID for use by resource providers.  It has the given optional prefix and
// the total length is capped to the maxlen.  Note that capping to maxlen necessarily increases the risk of collisions.
func NewUniqueHexID(prefix string, maxlen, randlen int) ID {
	return ID(NewUniqueHex(prefix, maxlen, randlen))
}
