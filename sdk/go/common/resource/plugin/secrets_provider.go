// Copyright 2016-2022, Pulumi Corporation.
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

package plugin

import (
	"context"
	"encoding/json"
	"io"
)

type Prompt struct {
	Preserve []string
	Label    string
	Text     string
	Error    string
}

type SecretsProvider interface {
	// Closer closes any underlying OS resources associated with this provider (like processes, RPC channels, etc).
	io.Closer

	Initalize(ctx context.Context, args []string, inputs map[string]string) (*Prompt, *json.RawMessage, error)

	Configure(ctx context.Context, state json.RawMessage, inputs map[string]string) (*Prompt, error)

	Encrypt(ctx context.Context, plaintexts []string) ([]string, error)

	Decrypt(ctx context.Context, ciphertexts []string) ([]string, error)
}
