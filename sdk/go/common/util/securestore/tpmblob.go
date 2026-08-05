// Copyright 2026, Pulumi Corporation.
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

package securestore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

// A TPM-sealed key is serialized as two length-prefixed chunks:
//
//	uint16 BE length | TPM2B_PRIVATE bytes | uint16 BE length | TPM2B_PUBLIC bytes
//
// Both TPM2B structures are self-describing, but the explicit lengths make
// parsing trivial and independent of TPM wire-format details. These helpers
// are pure so they can be unit-tested on any platform.

// encodeSealedBlob serializes the marshaled TPM2B_PRIVATE and TPM2B_PUBLIC
// parts of a sealed object.
func encodeSealedBlob(priv, pub []byte) ([]byte, error) {
	if len(priv) > math.MaxUint16 || len(pub) > math.MaxUint16 {
		return nil, fmt.Errorf("TPM sealed blob is implausibly large (%d/%d bytes)", len(priv), len(pub))
	}
	out := make([]byte, 0, 4+len(priv)+len(pub))
	out = binary.BigEndian.AppendUint16(out, uint16(len(priv))) //nolint:gosec // bounds checked above
	out = append(out, priv...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(pub))) //nolint:gosec // bounds checked above
	out = append(out, pub...)
	return out, nil
}

// decodeSealedBlob parses a blob produced by encodeSealedBlob back into its
// TPM2B_PRIVATE and TPM2B_PUBLIC parts.
func decodeSealedBlob(blob []byte) (priv, pub []byte, err error) {
	priv, rest, err := readSealedChunk(blob)
	if err != nil {
		return nil, nil, fmt.Errorf("stored key is corrupt (bad TPM blob): %w", err)
	}
	pub, rest, err = readSealedChunk(rest)
	if err != nil {
		return nil, nil, fmt.Errorf("stored key is corrupt (bad TPM blob): %w", err)
	}
	if len(rest) != 0 {
		return nil, nil, fmt.Errorf("stored key is corrupt (%d trailing bytes in TPM blob)", len(rest))
	}
	return priv, pub, nil
}

func readSealedChunk(b []byte) (chunk, rest []byte, err error) {
	if len(b) < 2 {
		return nil, nil, errors.New("truncated length prefix")
	}
	n := int(binary.BigEndian.Uint16(b))
	b = b[2:]
	if len(b) < n {
		return nil, nil, fmt.Errorf("declared %d bytes but only %d remain", n, len(b))
	}
	return b[:n], b[n:], nil
}
