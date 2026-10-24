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

// Package stackenv evaluates the definition of an ESC environment that is managed by a stack.
package stackenv

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/eval"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"gopkg.in/yaml.v3"
)

// Evaluation is the result of evaluating a stack's managed environment definition.
type Evaluation struct {
	Environment *esc.Environment
	// BaseRevision is the published revision the local definition was compared with.
	BaseRevision int
	// ChangeRequestID is set when the definition differs from BaseRevision.
	ChangeRequestID string
	// DraftRevision is the change request revision that was opened. It is meaningful only when
	// ChangeRequestID is set.
	DraftRevision int
}

// Evaluate opens the managed environment org/project/env with the given definition. If the definition equals the
// published one, the published revision is opened. Otherwise a draft is created and opened pinned to its revision 0.
func Evaluate(
	ctx context.Context,
	c client.Client,
	org string,
	project string,
	env string,
	definition []byte,
	duration time.Duration,
) (*Evaluation, []client.EnvironmentDiagnostic, error) {
	if err := rejectPlaintextSecrets(definition); err != nil {
		return nil, nil, err
	}

	published, etag, revision, err := c.GetEnvironment(ctx, org, project, env, "", false)
	if err != nil {
		if client.IsNotFound(err) {
			return nil, nil, fmt.Errorf(
				"managed environment %s/%s was not found; the stack cannot load its config", project, env)
		}
		return nil, nil, err
	}

	equal, err := DefinitionsEqual(definition, published)
	if err != nil {
		return nil, nil, err
	}

	evaluation := &Evaluation{BaseRevision: revision}
	var openID string
	var diags []client.EnvironmentDiagnostic
	if equal {
		openID, diags, err = c.OpenEnvironment(ctx, org, project, env, strconv.Itoa(revision), duration)
	} else {
		evaluation.ChangeRequestID, diags, err = c.CreateEnvironmentDraft(ctx, org, project, env, definition, etag)
		if err != nil {
			return nil, nil, fmt.Errorf("creating a draft of managed environment %s/%s: %w", project, env, err)
		}
		if len(diags) != 0 {
			return nil, diags, nil
		}
		evaluation.DraftRevision = 0
		openID, diags, err = c.OpenEnvironmentDraft(
			ctx, org, project, env, evaluation.ChangeRequestID, &evaluation.DraftRevision, duration)
	}
	if err != nil || len(diags) != 0 {
		return nil, diags, err
	}

	evaluation.Environment, err = c.GetOpenEnvironment(ctx, org, project, env, openID)
	if err != nil {
		return nil, nil, err
	}
	return evaluation, nil, nil
}

// DefinitionsEqual reports whether two environment definitions are equal after normalizing the formatting that
// the service rewrites on save. Comments are significant.
func DefinitionsEqual(local []byte, published []byte) (bool, error) {
	normalizedLocal, err := normalize(local)
	if err != nil {
		return false, fmt.Errorf("local definition: %w", err)
	}
	normalizedPublished, err := normalize(published)
	if err != nil {
		return false, fmt.Errorf("published definition: %w", err)
	}
	return bytes.Equal(normalizedLocal, normalizedPublished), nil
}

func normalize(definition []byte) ([]byte, error) {
	node, diags := encoding.DecodeYAMLBytes("", definition, eval.TagDecoder)
	if len(diags) != 0 {
		return nil, diags
	}
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if diags := encoding.EncodeYAML(enc, node); len(diags) != 0 {
		return nil, diags
	}
	return b.Bytes(), nil
}

// rejectPlaintextSecrets fails on `fn::secret: <string>`: the service encrypts it with a fresh ciphertext on every
// save, so such a definition would never compare equal to the published one.
func rejectPlaintextSecrets(definition []byte) error {
	node, diags := encoding.DecodeYAMLBytes("", definition, eval.TagDecoder)
	if len(diags) != 0 {
		return diags
	}
	var plaintextPath string
	_, _, err := syntax.Walk(node, func(n syntax.Node) (syntax.Node, syntax.Diagnostics, error) {
		obj, ok := n.(*syntax.ObjectNode)
		if !ok || plaintextPath != "" || obj.Len() != 1 || obj.Index(0).Key.Value() != "fn::secret" {
			return n, nil, nil
		}
		if _, ok := obj.Index(0).Value.(*syntax.StringNode); ok {
			plaintextPath = n.Syntax().Path()
		}
		return n, nil, nil
	})
	if err != nil {
		return err
	}
	if plaintextPath != "" {
		return fmt.Errorf(
			"replace plaintext secret at %s with `pulumi config set --secret`", plaintextPath)
	}
	return nil
}
