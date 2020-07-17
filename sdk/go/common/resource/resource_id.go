// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	cryptorand "crypto/rand"
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
)

// ID is a unique resource identifier; it is managed by the provider and is mostly opaque.
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

// NewUniqueHex generates a new "random" hex string for use by resource providers. It will take the optional prefix
// and append randlen random characters (defaulting to 8 if not > 0).  The result must not exceed maxlen total
// characterss (if > 0).  Note that capping to maxlen necessarily increases the risk of collisions.
func NewUniqueHex(prefix string, randlen, maxlen int) (string, error) {
	if randlen <= 0 {
		randlen = 8
	}
	if maxlen > 0 && len(prefix)+randlen > maxlen {
		return "", errors.Errorf(
			"name '%s' plus %d random chars is longer than maximum length %d", prefix, randlen, maxlen)
	}

	bs := make([]byte, randlen+1/2)
	n, err := cryptorand.Read(bs)
	contract.AssertNoError(err)
	contract.Assert(n == len(bs))

	return prefix + hex.EncodeToString(bs)[:randlen], nil
}

// NewUniqueHexID generates a new "random" hex string for use by resource providers. It will take the optional prefix
// and append randlen random characters (defaulting to 8 if not > 0).  The result must not exceed maxlen total
// characterss (if > 0).  Note that capping to maxlen necessarily increases the risk of collisions.
func NewUniqueHexID(prefix string, randlen, maxlen int) (ID, error) {
	u, err := NewUniqueHex(prefix, randlen, maxlen)
	return ID(u), err
}
