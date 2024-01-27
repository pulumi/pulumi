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

package sig

const (
	// SigKey is sometimes used to encode type identity inside of a map.  This is
	// required when flattening into ordinary maps, like we do when performing
	// serialization, to ensure recoverability of type identities later on.
	Key = "4dabf18193072939515e22adb298388d"

	// SecretSig is the unique secret signature.
	Secret = "1b47061264138c4ac30d75fd1eb44270"

	// ResourceReferenceSig is the unique resource reference signature.
	ResourceReference = "5cf8f73096256a8f31e491e813e4eb8e"

	// OutputValueSig is the unique output value signature.
	OutputValue = "d0e6a833031e9bbcd3f4e8bde6ca49a4"

	// a randomly assigned type hash for assets.
	AssetSig = "c44067f5952c0a294b673a41bacd8c17"

	// a randomly assigned archive type signature.
	ArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7"
)
