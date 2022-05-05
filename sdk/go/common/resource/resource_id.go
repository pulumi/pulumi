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
	"crypto"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"math/rand"

	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
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

	bs := make([]byte, (randlen+1)/2)
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

// NewUniqueHexV2 generates a new "random" hex string for use by resource providers. It will take the optional prefix
// and append randlen random characters (defaulting to 8 if not > 0).  The result must not exceed maxlen total
// characterss (if > 0).  Note that capping to maxlen necessarily increases the risk of collisions.
// The randomness for this method is a function of urn and sequenceNumber iff sequenceNUmber > 0, else it falls back to
// a non-deterministic source of randomness.
func NewUniqueHexV2(urn URN, sequenceNumber int, prefix string, randlen, maxlen int) (string, error) {
	if randlen <= 0 {
		randlen = 8
	}
	if maxlen > 0 && len(prefix)+randlen > maxlen {
		return "", errors.Errorf(
			"name '%s' plus %d random chars is longer than maximum length %d", prefix, randlen, maxlen)
	}

	if sequenceNumber == 0 {
		// No sequence number fallback to old logic
		return NewUniqueHex(prefix, randlen, maxlen)
	}

	if randlen > 32 {
		return "", errors.Errorf("randLen is longer than 32, %d", randlen)
	}

	// TODO(seqnum) This is seeded by urn and sequence number, and urn has the stack and project names in it.
	// But do we care about org name as well?
	// Do we need a config source of randomness so if users hit a collision they can set a config value to get out of it?
	hasher := crypto.SHA512.New()

	_, err := hasher.Write([]byte(urn))
	contract.AssertNoError(err)

	err = binary.Write(hasher, binary.LittleEndian, uint32(sequenceNumber))
	contract.AssertNoError(err)

	bs := hasher.Sum(nil)
	contract.Assert(len(bs) == 64)

	return prefix + hex.EncodeToString(bs)[:randlen], nil
}

type hashSource struct {
	hasher hash.Hash
}

func (hashSource) Seed(seed int64) {
	panic("you cannot seed the hash source")
}

func (h *hashSource) Int63() int64 {
	sofar := h.hasher.Sum(nil)
	random := binary.LittleEndian.Uint64(sofar)
	h.hasher.Write(sofar)
	result := random & ((1 << 63) - 1)
	return int64(result)
}

// NewUniqueString generates a new "random" string for use by resource providers. It will take the optional prefix
// and append randlen random characters (defaulting to 8 if not > 0). The result must not exceed maxlen total
// characters (if > 0).  Note that capping to maxlen necessarily increases the risk of collisions.
// The characters used for the random suffix are set by randChars and default to the set [a-f0-9] if nil.
// The randomness for this method is seeded by entropy.
func NewUniqueString(entropy []byte, prefix string, randlen, maxlen int, randchars []rune) (string, error) {
	if randlen <= 0 {
		randlen = 8
	}
	if maxlen > 0 && len(prefix)+randlen > maxlen {
		return "", errors.Errorf(
			"name '%s' plus %d random chars is longer than maximum length %d", prefix, randlen, maxlen)
	}

	if randchars == nil {
		randchars = []rune("0123456789abcdef")
	}

	hasher := crypto.SHA512.New()
	hasher.Write(entropy)
	source := &hashSource{hasher: hasher}
	rand := rand.New(source)

	b := make([]rune, randlen)
	for i := range b {
		b[i] = randchars[rand.Intn(len(randchars))]
	}

	return prefix + string(b), nil
}
