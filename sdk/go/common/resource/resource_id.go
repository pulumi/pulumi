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
	"fmt"

	"lukechampine.com/frand"

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
		return "", fmt.Errorf(
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
// and append randLen random characters (defaulting to 8 if not > 0).  The result must not exceed maxLen total
// characters (if > 0).  Note that capping to maxLen necessarily increases the risk of collisions.
// The randomness for this method is a function of urn and sequenceNumber iff sequenceNUmber > 0, else it falls back to
// a non-deterministic source of randomness.
func NewUniqueHexV2(urn URN, sequenceNumber int, prefix string, randLen, maxLen int) (string, error) {
	if randLen <= 0 {
		randLen = 8
	}
	if maxLen > 0 && len(prefix)+randLen > maxLen {
		return "", fmt.Errorf(
			"name '%s' plus %d random chars is longer than maximum length %d", prefix, randLen, maxLen)
	}

	if sequenceNumber == 0 {
		// No sequence number fallback to old logic
		return NewUniqueHex(prefix, randLen, maxLen)
	}

	if randLen > 32 {
		return "", fmt.Errorf("randLen is longer than 32, %d", randLen)
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

	return prefix + hex.EncodeToString(bs)[:randLen], nil
}

// NewUniqueName generates a new "random" string primarily intended for use by resource providers for
// autonames. It will take the optional prefix and append randlen random characters (defaulting to 8 if not >
// 0). The result must not exceed maxlen total characters (if > 0). The characters that make up the random
// suffix can be set via charset, and will default to [a-f0-9]. Note that capping to maxlen necessarily
// increases the risk of collisions. The randomness for this method is a function of randomSeed if given, else
// it falls back to a non-deterministic source of randomness.
func NewUniqueName(randomSeed []byte, prefix string, randlen, maxlen int, charset []rune) (string, error) {
	if randlen <= 0 {
		randlen = 8
	}
	if maxlen > 0 && len(prefix)+randlen > maxlen {
		return "", fmt.Errorf(
			"name '%s' plus %d random chars is longer than maximum length %d", prefix, randlen, maxlen)
	}

	if charset == nil {
		charset = []rune("0123456789abcdef")
	}

	var random *frand.RNG
	if len(randomSeed) == 0 {
		random = frand.New()
	} else {
		// frand.NewCustom needs a 32 byte seed. Take the SHA256 hash of whatever bytes we've been given as a
		// seed and pass the 32 byte result of that to frand.
		hash := crypto.SHA256.New()
		hash.Write(randomSeed)
		seed := hash.Sum(nil)
		bufsize := 1024 // Same bufsize as used by frand.New.
		rounds := 12    // Same rounds as used by frand.New.
		random = frand.NewCustom(seed, bufsize, rounds)
	}

	randomSuffix := make([]rune, randlen)
	for i := range randomSuffix {
		randomSuffix[i] = charset[random.Intn(len(charset))]
	}

	return prefix + string(randomSuffix), nil
}
