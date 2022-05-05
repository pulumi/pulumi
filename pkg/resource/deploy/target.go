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

package deploy

import (
	"crypto"
	"encoding/binary"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Target represents information about a deployment target.
type Target struct {
	Name      tokens.Name      // the target stack name.
	Config    config.Map       // optional configuration key/value pairs.
	Decrypter config.Decrypter // decrypter for secret configuration values.
	Snapshot  *Snapshot        // the last snapshot deployed to the target.
	Entropy   []byte           // the entropy for this target.
}

// Build the entropy for a resource call, this is a 512 bit hash of the stack entropy, the urn and the sequence number
func (t *Target) GetEntropy(urn resource.URN, sequenceNumber int) []byte {
	hasher := crypto.SHA512.New()
	_, err := hasher.Write(t.Entropy)
	contract.AssertNoError(err)
	_, err = hasher.Write([]byte(urn))
	contract.AssertNoError(err)
	err = binary.Write(hasher, binary.LittleEndian, uint32(sequenceNumber))
	contract.AssertNoError(err)
	entropy := hasher.Sum(nil)
	contract.Assert(len(entropy) == 64)
	return entropy
}

// GetPackageConfig returns the set of configuration parameters for the indicated package, if any.
func (t *Target) GetPackageConfig(pkg tokens.Package) (resource.PropertyMap, error) {
	result := resource.PropertyMap{}
	if t == nil {
		return result, nil
	}

	for k, c := range t.Config {
		if tokens.Package(k.Namespace()) != pkg {
			continue
		}

		v, err := c.Value(t.Decrypter)
		if err != nil {
			return nil, err
		}

		propertyValue := resource.NewStringProperty(v)
		if c.Secure() {
			propertyValue = resource.MakeSecret(propertyValue)
		}
		result[resource.PropertyKey(k.Name())] = propertyValue
	}
	return result, nil
}
