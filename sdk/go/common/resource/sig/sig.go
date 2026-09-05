// Copyright 2016, Pulumi Corporation.
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

	// ByteString is the unique signature for string values that contain bytes which are not
	// valid UTF-8. Such strings cannot be transported as protobuf string fields, so they are
	// encoded as an object carrying the base64 encoding of the string's bytes.
	ByteString = "803fd3297a5875dc03ca845dda5d2a98"

	// a randomly assigned type hash for assets.
	AssetSig = "c44067f5952c0a294b673a41bacd8c17"

	// a randomly assigned archive type signature.
	ArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7"
)

// Unknown values are transported as sentinel strings. Each sentinel records the type of the value
// that is not yet known, so that the receiver can recover the type.
const (
	// UnknownBoolValue is the sentinel for a bool value that is not known.
	UnknownBoolValue = "1c4a061d-8072-4f0a-a4cb-0ff528b18fe7"

	// UnknownNumberValue is the sentinel for a number value that is not known.
	UnknownNumberValue = "3eeb2bf0-c639-47a8-9e75-3b44932eb421"

	// UnknownStringValue is the sentinel for a string value that is not known.
	UnknownStringValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

	// UnknownArrayValue is the sentinel for an array value that is not known.
	UnknownArrayValue = "6a19a0b0-7e62-4c92-b797-7f8e31da9cc2"

	// UnknownAssetValue is the sentinel for an asset value that is not known.
	UnknownAssetValue = "030794c1-ac77-496b-92df-f27374a8bd58"

	// UnknownArchiveValue is the sentinel for an archive value that is not known.
	UnknownArchiveValue = "e48ece36-62e2-4504-bad9-02848725956a"

	// UnknownObjectValue is the sentinel for an object value that is not known.
	UnknownObjectValue = "dd056dcd-154b-4c76-9bd3-c8f88648b5ff"
)

// IsUnknown reports if s is one of the sentinels that describe a value that is not known.
func IsUnknown(s string) bool {
	switch s {
	case UnknownBoolValue, UnknownNumberValue, UnknownStringValue, UnknownArrayValue,
		UnknownAssetValue, UnknownArchiveValue, UnknownObjectValue:
		return true
	default:
		return false
	}
}
