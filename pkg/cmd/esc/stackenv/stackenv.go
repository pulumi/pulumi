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
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/eval"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
	if pulumiConfig, ok := objectProperty(objectProperty(node, "values"), "pulumiConfig").(*syntax.ObjectNode); ok {
		for i := range pulumiConfig.Len() {
			entry := pulumiConfig.Index(i)
			if secret, path := findPlaintextSecret(entry.Value, nil); secret != nil {
				return fmt.Errorf("%s is a plaintext secret; set it with `%s`",
					secret.Syntax().Path(), configSetSecretCommand(entry.Key.Value(), path))
			}
		}
	}
	if secret, _ := findPlaintextSecret(node, nil); secret != nil {
		return fmt.Errorf("%s is a plaintext secret; set it under values.pulumiConfig with "+
			"`pulumi config set --secret` and reference it from there", secret.Syntax().Path())
	}
	return nil
}

func objectProperty(node syntax.Node, key string) syntax.Node {
	obj, ok := node.(*syntax.ObjectNode)
	if !ok {
		return nil
	}
	for i := range obj.Len() {
		if obj.Index(i).Key.Value() == key {
			return obj.Index(i).Value
		}
	}
	return nil
}

func findPlaintextSecret(node syntax.Node, path resource.PropertyPath) (syntax.Node, resource.PropertyPath) {
	switch node := node.(type) {
	case *syntax.ObjectNode:
		if node.Len() == 1 && node.Index(0).Key.Value() == "fn::secret" {
			if _, ok := node.Index(0).Value.(*syntax.StringNode); ok {
				return node, path
			}
			return nil, nil
		}
		for i := range node.Len() {
			entry := node.Index(i)
			entryPath := slices.Concat(path, resource.PropertyPath{entry.Key.Value()})
			if secret, secretPath := findPlaintextSecret(entry.Value, entryPath); secret != nil {
				return secret, secretPath
			}
		}
	case *syntax.ArrayNode:
		for i := range node.Len() {
			elementPath := slices.Concat(path, resource.PropertyPath{i})
			if secret, secretPath := findPlaintextSecret(node.Index(i), elementPath); secret != nil {
				return secret, secretPath
			}
		}
	}
	return nil, nil
}

func configSetSecretCommand(configKey string, path resource.PropertyPath) string {
	if len(path) == 0 {
		return fmt.Sprintf("pulumi config set --secret %s <value>", configKey)
	}
	namespace, name, namespaced := strings.Cut(configKey, ":")
	if !namespaced {
		namespace, name = "", configKey
	}
	keyPath := append(resource.PropertyPath{name}, path...).String()
	if namespaced {
		keyPath = namespace + ":" + keyPath
	}
	return fmt.Sprintf("pulumi config set --secret --path '%s' <value>", keyPath)
}
