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

package stack

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdCmd "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/cmd"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// stackJSONEnvelope is the stable, machine-readable shape emitted by
// `pulumi stack --output json` (and its alias, `pulumi stack get`). New
// fields may be added; existing field names and types should not change.
//
// Time values are formatted with cmd.FormatTime (RFC 5424 with millisecond
// precision, UTC) to match the convention already established by
// `pulumi stack history --json` and `pulumi stack ls --json`.
type stackJSONEnvelope struct {
	Organization     string              `json:"organization,omitempty"`
	Project          string              `json:"project,omitempty"`
	Stack            string              `json:"stack"`
	Backend          string              `json:"backend"`
	Version          *int                `json:"version,omitempty"`
	ActiveUpdate     string              `json:"activeUpdate,omitempty"`
	CurrentOperation *stackOperationJSON `json:"currentOperation,omitempty"`
	Tags             map[string]string   `json:"tags"`
	Manifest         *stackManifestJSON  `json:"manifest,omitempty"`
	Resources        []stackResourceJSON `json:"resources"`
	Outputs          map[string]any      `json:"outputs"`
	ConsoleURL       string              `json:"consoleUrl,omitempty"`
}

type stackOperationJSON struct {
	Kind    string `json:"kind"`
	Author  string `json:"author"`
	Started string `json:"started"`
}

type stackManifestJSON struct {
	Time          string            `json:"time,omitempty"`
	PulumiVersion string            `json:"pulumiVersion,omitempty"`
	Plugins       []stackPluginJSON `json:"plugins,omitempty"`
}

type stackPluginJSON struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Version string `json:"version,omitempty"`
}

type stackResourceJSON struct {
	URN    string `json:"urn"`
	Type   string `json:"type"`
	Name   string `json:"name"`
	ID     string `json:"id,omitempty"`
	Parent string `json:"parent,omitempty"`
}

// stackJSONInputs bundles the resolved inputs for buildStackJSON. Splitting
// it out keeps the builder pure: it has no I/O and no dependency on the
// backend interface, so tests can populate the struct directly.
type stackJSONInputs struct {
	// StackName is the short stack name (no org/project prefix). This is what
	// the `stack` field of the JSON envelope is set to.
	StackName string
	// Project is the project name. May be overridden by the cloud-side value.
	Project string
	// BackendName names the backend the stack lives on (e.g. "pulumi.com").
	BackendName string

	// CloudStack carries cloud-only metadata from GetStack. nil on DIY
	// backends or when the lookup wasn't performed.
	CloudStack *apitype.Stack
	// ConsoleURL is the Pulumi Cloud console link for this stack. Empty on
	// DIY backends.
	ConsoleURL string

	// Snapshot is the locally-loaded deployment snapshot. nil if it could
	// not be loaded (e.g. no local project).
	Snapshot *deploy.Snapshot
	// ShowSecrets controls whether stack output secrets are unmasked.
	ShowSecrets bool
}

// buildStackJSON assembles a stackJSONEnvelope from the resolved inputs.
// Pure — no I/O.
func buildStackJSON(in stackJSONInputs) *stackJSONEnvelope {
	env := &stackJSONEnvelope{
		Project:   in.Project,
		Stack:     in.StackName,
		Backend:   in.BackendName,
		Tags:      map[string]string{},
		Resources: []stackResourceJSON{},
		Outputs:   map[string]any{},
	}

	if cs := in.CloudStack; cs != nil {
		env.Organization = cs.OrgName
		if cs.ProjectName != "" {
			env.Project = cs.ProjectName
		}
		v := cs.Version
		env.Version = &v
		env.ActiveUpdate = cs.ActiveUpdate
		for k, v := range cs.Tags {
			env.Tags[k] = v
		}
		if op := cs.CurrentOperation; op != nil {
			env.CurrentOperation = &stackOperationJSON{
				Kind:    string(op.Kind),
				Author:  op.Author,
				Started: cmdCmd.FormatTime(time.Unix(op.Started, 0).UTC()),
			}
		}
	}
	env.ConsoleURL = in.ConsoleURL

	if snap := in.Snapshot; snap != nil {
		manifestTime := ""
		if !snap.Manifest.Time.IsZero() {
			manifestTime = cmdCmd.FormatTime(snap.Manifest.Time.UTC())
		}
		env.Manifest = &stackManifestJSON{
			Time:          manifestTime,
			PulumiVersion: snap.Manifest.Version,
		}
		for _, p := range snap.Manifest.Plugins {
			version := ""
			if p.Version != nil {
				version = p.Version.String()
			}
			env.Manifest.Plugins = append(env.Manifest.Plugins, stackPluginJSON{
				Name:    p.Name,
				Kind:    string(p.Kind),
				Version: version,
			})
		}

		env.Resources = make([]stackResourceJSON, 0, len(snap.Resources))
		for _, r := range snap.Resources {
			env.Resources = append(env.Resources, snapshotResourceJSON(r))
		}

		if outs, err := getStackOutputs(snap, in.ShowSecrets); err == nil {
			env.Outputs = outs
		}
	}

	return env
}

func snapshotResourceJSON(r *resource.State) stackResourceJSON {
	return stackResourceJSON{
		URN:    string(r.URN),
		Type:   string(r.Type),
		Name:   r.URN.Name(),
		ID:     string(r.ID),
		Parent: string(r.Parent),
	}
}

// loadStackJSONInputs assembles the inputs buildStackJSON needs from a
// resolved backend.Stack: it best-effort loads the local snapshot and, on
// Pulumi Cloud backends, fetches the GetStack metadata.
func loadStackJSONInputs(
	ctx context.Context, s backend.Stack, showSecrets bool,
) (stackJSONInputs, error) {
	ref := s.Ref()
	project := ""
	if p, ok := ref.Project(); ok {
		project = string(p)
	}

	in := stackJSONInputs{
		StackName:   ref.Name().String(),
		Project:     project,
		BackendName: s.Backend().Name(),
		ShowSecrets: showSecrets,
	}

	cloudInfo, consoleURL, err := fetchCloudStackInfo(ctx, s)
	if err != nil {
		return stackJSONInputs{}, err
	}
	in.CloudStack = cloudInfo
	in.ConsoleURL = consoleURL

	snap, err := s.Snapshot(ctx, secrets.DefaultProvider)
	if err != nil {
		return stackJSONInputs{}, err
	}
	// snap may legitimately be nil (no updates yet); buildStackJSON handles
	// that and just omits the manifest/resources/outputs sections.
	in.Snapshot = snap

	return in, nil
}

// fetchCloudStackInfo fetches the cloud-only stack metadata (version, tags,
// activeUpdate, currentOperation) and console URL for s. Returns
// (nil, "", nil) when s isn't on the Pulumi Cloud backend.
func fetchCloudStackInfo(
	ctx context.Context, s backend.Stack,
) (*apitype.Stack, string, error) {
	cloudStack, ok := s.(httpstate.Stack)
	if !ok {
		return nil, "", nil
	}
	be := cloudStack.Backend().(httpstate.Backend)
	info, err := be.Client().GetStack(ctx, cloudStack.StackIdentifier())
	if err != nil {
		return nil, "", err
	}
	consoleURL := ""
	if u, urlErr := be.StackConsoleURL(s.Ref()); urlErr == nil {
		consoleURL = u
	}
	return &info, consoleURL, nil
}

func renderStackJSON(w io.Writer, env *stackJSONEnvelope) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}
