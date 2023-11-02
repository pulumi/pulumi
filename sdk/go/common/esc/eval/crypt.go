// Copyright 2023, Pulumi Corporation.
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

package eval

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/esc/syntax/encoding"
	"gopkg.in/yaml.v3"
)

// An Encrypter encrypts plaintext into ciphertext.
type Encrypter interface {
	// Encrypt encrypts a single plaintext value.
	Encrypt(ctx context.Context, value []byte) ([]byte, error)
}

// A Decrypter decrypts ciphertext into plaintext.
type Decrypter interface {
	// Decrypt decrypts a single ciphertext value.
	Decrypt(ctx context.Context, value []byte) ([]byte, error)
}

// rewriteYAML is a helper for rewriting a single YAML document.
func rewriteYAML(
	ctx context.Context,
	filename string,
	source []byte,
	visitor func(n syntax.Node) (syntax.Node, syntax.Diagnostics, error),
) ([]byte, error) {
	syn, diags := encoding.DecodeYAMLBytes(filename, source, TagDecoder)
	if len(diags) != 0 {
		return nil, diags
	}

	doc, diags, err := syntax.Walk(syn, visitor)
	if err != nil {
		return nil, err
	}
	if len(diags) != 0 {
		return nil, diags
	}

	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	diags = encoding.EncodeYAML(enc, doc)
	if len(diags) != 0 {
		return nil, diags
	}
	return b.Bytes(), nil
}

// parseSecret attempts to parse a syntax.Node as a call to the fn::secret builtin. If the node is such a call,
// parseSecret extracts and returns the plaintext or ciphertext.
//
// A call that carries ciphertext is of the form
//
//	fn::secret:
//	  ciphertext: <base64-encoded envelope>
//
// A call that carries plaintext is of the form
//
//	fn::secret: <string literal>
func parseSecret(node syntax.Node) (obj *syntax.ObjectNode, plaintext, ciphertext *syntax.StringNode, ok bool) {
	obj, ok = node.(*syntax.ObjectNode)
	if !ok {
		return nil, nil, nil, false
	}
	if obj.Len() != 1 || obj.Index(0).Key.Value() != "fn::secret" {
		return nil, nil, nil, false
	}
	value := obj.Index(0).Value

	if arg, ok := value.(*syntax.ObjectNode); ok && arg.Len() == 1 {
		kvp := arg.Index(0)
		if kvp.Key.Value() == "ciphertext" {
			if str, ok := kvp.Value.(*syntax.StringNode); ok {
				return obj, nil, str, true
			}
		}
	}

	str, ok := value.(*syntax.StringNode)
	if !ok {
		return nil, nil, nil, false
	}
	return obj, str, nil, true
}

// EncryptSecrets encrypts any secrets in the given YAML document and returns the rewritten source. Encryption replaces
// all plaintext arguments to `fn::secret` with encrypted ciphertext.
func EncryptSecrets(ctx context.Context, filename string, source []byte, encrypter Encrypter) ([]byte, error) {
	return rewriteYAML(ctx, filename, source, func(n syntax.Node) (syntax.Node, syntax.Diagnostics, error) {
		obj, plaintext, _, ok := parseSecret(n)
		if !ok || plaintext == nil {
			return n, nil, nil
		}

		// Encrypt the plaintext.
		ciphertext, err := encrypter.Encrypt(ctx, []byte(plaintext.Value()))
		if err != nil {
			return nil, nil, err
		}

		// Replace the original call to `fn::secret` with a new call whose argument is the encrypted ciphertext.
		//
		// Trivia from the plaintext value is copied to the ciphertext string.
		return syntax.ObjectSyntax(obj.Syntax(),
			syntax.ObjectPropertySyntax(
				obj.Index(0).Syntax,
				obj.Index(0).Key,
				syntax.Object(
					syntax.ObjectProperty(
						syntax.String("ciphertext"),
						syntax.StringSyntax(syntax.CopyTrivia(plaintext.Syntax()), encodeCiphertext(ciphertext)),
					),
				),
			),
		), nil, nil
	})
}

// DecryptSecrets decrypts any secrets in the given YAML document and returns the rewritten source. Decryption replaces
// all ciphertext arguments to `fn::secret` with decrypted plaintext.
func DecryptSecrets(ctx context.Context, filename string, source []byte, decrypter Decrypter) ([]byte, error) {
	return rewriteYAML(ctx, filename, source, func(n syntax.Node) (syntax.Node, syntax.Diagnostics, error) {
		obj, _, ciphertextNode, ok := parseSecret(n)
		if !ok || ciphertextNode == nil {
			return n, nil, nil
		}

		ciphertext, err := decodeCiphertext(ciphertextNode.Value())
		if err != nil {
			return nil, nil, fmt.Errorf("invalid ciphertext: %w", err)
		}

		plaintext, err := decrypter.Decrypt(ctx, ciphertext)
		if err != nil {
			return nil, nil, err
		}

		// Replace the original call to `fn::secret` with a new call whose argument is the decrypted plaintext.
		return syntax.ObjectSyntax(obj.Syntax(),
			syntax.ObjectPropertySyntax(obj.Index(0).Syntax, obj.Index(0).Key, syntax.StringSyntax(ciphertextNode.Syntax(), string(plaintext))),
		), nil, nil
	})
}

const envelopeMagic = "escx"
const envelopeVersion = uint32(1)

// Ciphertext is wrapped in an envelope that is encoded as a binary string of the form
//
//	“escx” <envelope version> <ciphertext> <crc32>
//
// The envelope version and checksum are encoded as big-endian 4-byte values. The checksum takes into account all of
// the preceding bytes. It is highly unlikely that user-specified plaintext values will collide with this encoding.
//
// The envelope itself is base64-encoded.
func decodeCiphertext(repr string) ([]byte, error) {
	bin, err := base64.StdEncoding.DecodeString(repr)
	if err != nil {
		return nil, err
	}

	// The minimum length for the envelope is 16 bytes (4 bytes each for the magic number, version, length, and
	// checksum)
	if len(bin) < 16 {
		return nil, io.EOF
	}

	// The envelope must begin with "escx".
	if string(bin[0:4]) != envelopeMagic {
		return nil, errors.New("invalid header")
	}

	// The expected and actual checksums must match.
	expectedChecksum := binary.BigEndian.Uint32(bin[len(bin)-4:])
	actualChecksum := crc32.Checksum(bin[:len(bin)-4], crc32.IEEETable)
	if actualChecksum != expectedChecksum {
		return nil, fmt.Errorf("invalid checksum")
	}

	// The expected and actual versions must match.
	version := binary.BigEndian.Uint32(bin[4:])
	if version != envelopeVersion {
		return nil, fmt.Errorf("unsupported version")
	}

	// Extract the ciphertext.
	return bin[8 : len(bin)-4], nil
}

func encodeCiphertext(ciphertext []byte) string {
	var b bytes.Buffer
	b.WriteString(envelopeMagic)                                                            // "escx"
	b.Write(binary.BigEndian.AppendUint32(nil, envelopeVersion))                            // version
	b.Write(ciphertext)                                                                     // ciphertext
	b.Write(binary.BigEndian.AppendUint32(nil, crc32.Checksum(b.Bytes(), crc32.IEEETable))) // crc32
	return base64.StdEncoding.EncodeToString(b.Bytes())
}
