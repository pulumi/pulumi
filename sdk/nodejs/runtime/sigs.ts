// Copyright 2016-2026, Pulumi Corporation.
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

// Special-signature constants used to encode type identity inside resource
// property maps over the wire. Kept in a leaf module (no side-imports) so
// consumers — notably `@pulumi/policy` — can read them without dragging in
// the rest of the runtime tree (`output`, `resource`, `log`, `settings`,
// `state`, `semver`, …).

/**
 * {@link unknownValue} is a sentinel string used to represent unknown property
 * values during preview.
 */
export const unknownValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9";

/**
 * {@link specialSigKey} is sometimes used to encode type identity inside of a
 * map.
 *
 * @see sdk/go/common/resource/properties.go.
 */
export const specialSigKey = "4dabf18193072939515e22adb298388d";

/**
 * {@link specialAssetSig} is a randomly assigned hash used to identify assets
 * in maps.
 *
 * @see sdk/go/common/resource/asset.go.
 */
export const specialAssetSig = "c44067f5952c0a294b673a41bacd8c17";

/**
 * {@link specialArchiveSig} is a randomly assigned hash used to identify
 * archives in maps.
 *
 * @see sdk/go/common/resource/asset.go.
 */
export const specialArchiveSig = "0def7320c3a5731c473e5ecbe6d01bc7";

/**
 * {@link specialSecretSig} is a randomly assigned hash used to identify secrets
 * in maps.
 *
 * @see sdk/go/common/resource/properties.go.
 */
export const specialSecretSig = "1b47061264138c4ac30d75fd1eb44270";

/**
 * {@link specialResourceSig} is a randomly assigned hash used to identify
 * resources in maps.
 *
 * @see sdk/go/common/resource/properties.go.
 */
export const specialResourceSig = "5cf8f73096256a8f31e491e813e4eb8e";

/**
 * {@link specialOutputValueSig} is a randomly assigned hash used to identify
 * outputs in maps.
 *
 * @see sdk/go/common/resource/properties.go.
 */
export const specialOutputValueSig = "d0e6a833031e9bbcd3f4e8bde6ca49a4";
