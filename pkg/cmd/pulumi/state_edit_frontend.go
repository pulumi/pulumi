// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	dyff "github.com/dixler/dyff/pkg/pulumi"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	yaml "gopkg.in/yaml.v3"
)

type stateFrontend interface {
	GetBackingFile() string
	Reset() error
	SaveToFile(state apitype.DeploymentV3) error
	ReadNewSnapshot() (deploy.Snapshot, error)
	Diff() (string, error)
}

type jsonStateFrontend struct {
	backingFile string
	original    *apitype.DeploymentV3
}

var _ stateFrontend = &jsonStateFrontend{}

func (se *jsonStateFrontend) GetBackingFile() string {
	return ".pulumi/state-edit.json"
}

func (se *jsonStateFrontend) Reset() error {
	contract.Assert(se.original != nil)
	return se.SaveToFile(*se.original)
}

func (se *jsonStateFrontend) SaveToFile(state apitype.DeploymentV3) error {
	if se.original == nil {
		se.original = &state
	}
	if se.backingFile == "" {
		se.backingFile = ".pulumi/state-edit.json"
	}
	writer, err := os.Create(se.backingFile)
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "    ")

	if err = enc.Encode(state); err != nil {
		return fmt.Errorf("could not serialize deployment as JSON : %w", err)
	}
	return nil
}

func (se *jsonStateFrontend) ReadNewSnapshot() (deploy.Snapshot, error) {
	b, err := os.ReadFile(se.backingFile)
	if err != nil {
		return deploy.Snapshot{}, err
	}
	ctx := context.Background()
	dep, err := stack.DeserializeUntypedDeployment(ctx, &apitype.UntypedDeployment{
		Version:    3,
		Deployment: b,
	}, stack.DefaultSecretsProvider)
	if err != nil {
		return deploy.Snapshot{}, err
	}

	if dep == nil {
		return deploy.Snapshot{}, errors.New("could not deserialize deployment")
	}
	return *dep, nil
}

func (se *jsonStateFrontend) Diff() (string, error) {
	contract.Assert(se.original != nil)
	old := jsonStateFrontend{backingFile: ".pulumi/state-edit.original.json"}
	old.SaveToFile(*se.original)

	diff, err := dyff.Compare(old.backingFile, se.backingFile)
	if err != nil {
		return "", err
	}
	return diff, nil
}

type yamlStateFrontend struct {
	// Descending Historical State files
	backingFile string
	original    *apitype.DeploymentV3
}

var _ stateFrontend = &yamlStateFrontend{}

func (se *yamlStateFrontend) GetBackingFile() string {
	return ".pulumi/state-edit.yaml"
}

func (se *yamlStateFrontend) Reset() error {
	contract.Assert(se.original != nil)
	return se.SaveToFile(*se.original)
}
func writeYAMLHeader(w io.Writer) {
	w.Write([]byte("# Welcome to pulumi state edit!\n"))
	w.Write([]byte("# \"You've met with a terrible fate, haven't you?\"\n"))
}

type nodes []*yaml.Node

func (i nodes) Len() int { return len(i) / 2 }

func (i nodes) Swap(x, y int) {
	x *= 2
	y *= 2
	i[x], i[y] = i[y], i[x]         // keys
	i[x+1], i[y+1] = i[y+1], i[x+1] // values
}

func (i nodes) Less(x, y int) bool {
	x *= 2
	y *= 2
	xKey := i[x].Value
	yKey := i[y].Value
	priority := func(key string) int {
		p, ok := map[string]int{
			"urn":                     0,
			"custom":                  1,
			"delete":                  2,
			"id":                      3,
			"type":                    4,
			"inputs":                  5,
			"outputs":                 6,
			"parent":                  7,
			"protect":                 8,
			"external":                9,
			"dependencies":            10,
			"initErrors":              11,
			"provider":                12,
			"propertyDependencies":    13,
			"pendingReplacement":      14,
			"additionalSecretOutputs": 15,
			"aliases":                 16,
			"customTimeouts":          17,
			"importID":                18,
			"retainOnDelete":          19,
			"deletedWith":             20,
			"created":                 21,
			"modified":                22,
		}[key]
		if !ok {
			return 1000
		}
		return p
	}
	if priority(xKey) == priority(yKey) {
		return xKey < yKey
	}
	return priority(xKey) < priority(yKey)
}

func sortYAML(doc []byte) ([]byte, error) {
	var node yaml.Node
	err := yaml.Unmarshal([]byte(doc), &node)
	if err != nil {
		return []byte{}, err
	}
	res := sortYAMLInner(&node)
	b, err := yaml.Marshal(res)
	return b, err
}

func sortYAMLInner(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode {
		for i, n := range node.Content {
			node.Content[i] = sortYAMLInner(n)
		}
	}
	if node.Kind == yaml.SequenceNode {
		for i, n := range node.Content {
			node.Content[i] = sortYAMLInner(n)
		}
	}
	if node.Kind == yaml.MappingNode {
		for i, n := range node.Content {
			node.Content[i] = sortYAMLInner(n)
		}
		sort.Sort(nodes(node.Content))
	}
	return node
}

func (se *yamlStateFrontend) SaveToFile(state apitype.DeploymentV3) error {
	// TODO write constructor
	if se.original == nil {
		se.original = &state
	}
	if se.backingFile == "" {
		se.backingFile = ".pulumi/state-edit.yaml"
	}
	b, err := json.Marshal(state)
	contract.AssertNoError(err)

	idk := map[string]interface{}{}
	err = json.Unmarshal(b, &idk)
	contract.AssertNoError(err)

	writer := &bytes.Buffer{}
	if err != nil {
		return fmt.Errorf("could not open file: %w", err)
	}

	writeYAMLHeader(writer)

	enc := yaml.NewEncoder(writer)
	enc.SetIndent(2)
	if err = enc.Encode(idk); err != nil {
		return fmt.Errorf("could not serialize deployment as YAML : %w", err)
	}

	{
		wFile, err := os.Create(se.backingFile)
		if err != nil {
			return err
		}
		defer wFile.Close()

		b, err := sortYAML(writer.Bytes())
		if err != nil {
			return err
		}
		_, err = wFile.Write(b)
		if err != nil {
			return err
		}
		return nil
	}
}

func (se *yamlStateFrontend) ReadNewSnapshot() (deploy.Snapshot, error) {
	b, err := os.ReadFile(se.backingFile)
	if err != nil {
		return deploy.Snapshot{}, err
	}
	idk := map[string]interface{}{}
	err = yaml.Unmarshal(b, &idk)
	if err != nil {
		return deploy.Snapshot{}, err
	}

	bJson, err := json.Marshal(idk)
	if err != nil {
		return deploy.Snapshot{}, err
	}

	ctx := context.Background()
	dep, err := stack.DeserializeUntypedDeployment(ctx, &apitype.UntypedDeployment{
		Version:    3,
		Deployment: bJson,
	}, stack.DefaultSecretsProvider)
	if err != nil {
		return deploy.Snapshot{}, err
	}

	if dep == nil {
		return deploy.Snapshot{}, errors.New("could not deserialize deployment")
	}
	return *dep, nil
}
func (se *yamlStateFrontend) Diff() (string, error) {
	contract.Assert(se.original != nil)
	old := yamlStateFrontend{original: se.original, backingFile: ".pulumi/state-edit.original.yaml"}
	old.SaveToFile(*se.original)

	diff, err := dyff.Compare(old.backingFile, se.backingFile)
	if err != nil {
		return "", err
	}
	return diff, nil
}
