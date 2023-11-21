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

package cli

import (
	"context"
	"io"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/auto/events"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type PulumiCommands interface {
	Debug(opts *DebugOptions) PulumiCommands

	Stack(name string) StackCommands
	Plugin() PluginCommands

	WhoAmI(ctx context.Context) (*WhoAmIResult, error)
	Version(ctx context.Context) (*semver.Version, error)
}

type PluginCommands interface {
	Install(ctx context.Context, name, version string, opts *PluginInstallOptions) error
	Rm(ctx context.Context, name, version string)
	Ls(ctx context.Context) ([]workspace.PluginInfo, error)
}

type StackCommands interface {
	Config() ConfigCommands
	Tag() TagCommands

	Preview(ctx context.Context, opts *StackPreviewOptions) (*StackPreviewResult, error)
	Update(ctx context.Context, opts *StackUpdateOptions) (*StackUpdateResult, error)
	Refresh(ctx context.Context, opts *StackRefreshOptions) (*StackRefreshResult, error)
	Destroy(ctx context.Context, opts *StackDestroyOptions) (*StackDestroyResult, error)
	Cancel(ctx context.Context) error

	Init(ctx context.Context, opts *StackInitOptions) error
	Select(ctx context.Context) error
	Rm(ctx context.Context, opts *StackRmOptions) error
	Ls(ctx context.Context) ([]StackSummary, error)

	Export(ctx context.Context) (apitype.UntypedDeployment, error)
	Import(ctx context.Context, state apitype.UntypedDeployment) error
	Outputs(ctx context.Context) (map[string]OutputValue, error)
	History(ctx context.Context, opts *StackHistoryOptions) ([]UpdateSummary, error)
}

type ConfigCommands interface {
	Get(ctx context.Context, key string, opts *ConfigGetOptions) (ConfigValue, error)
	Set(ctx context.Context, key string, value ConfigValue, opts *ConfigSetOptions) error
	GetAll(ctx context.Context) (map[string]ConfigValue, error)
	SetAll(ctx context.Context, values map[string]ConfigValue) error
	Rm(ctx context.Context, key string, opts *ConfigRmOptions) error
	Refresh(ctx context.Context) error
}

type TagCommands interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Rm(ctx context.Context, key string) error
	Ls(ctx context.Context) (map[string]string, error)
}

type DebugOptions struct {
	LogToStderr bool
	LogLevel    int
	LogFlow     bool
	Tracing     string
	Debug       bool
}

type WhoAmIResult struct {
	User          string   `json:"user"`
	Organizations []string `json:"organizations,omitempty"`
	URL           string   `json:"url"`
}

type PluginInstallOptions struct {
	Server string
}

type EnvVarValue struct {
	Value  string
	Secret bool
}

type StackRemoteOptions struct {
	URL                     string
	Branch                  string
	CommitHash              string
	ProjectPath             string
	Auth                    *StackRemoteAuth
	PreRunCommands          []string
	EnvVars                 []EnvVarValue
	SkipInstallDependencies bool
}

type StackRemoteAuth struct {
	PersonalAccessToken string
	SSHPrivateKey       string
	SSHPrivateKeyPath   string
	Password            string
	Username            string
}

type StackStreams struct {
	Events        []chan<- events.EngineEvent
	Progress      []io.Writer
	ErrorProgress []io.Writer
}

type StackPreviewOptions struct {
	Message           string
	ExpectNoChanges   bool
	Diff              bool
	Replace           []string
	Target            []string
	PolicyPacks       []string
	PolicyPackConfigs []string
	TargetDependents  bool
	Parallel          int
	UserAgent         string
	Color             string
	Plan              string
	Client            string
	ExecKind          string

	Streams StackStreams
	Remote  *StackRemoteOptions
}

type StackPreviewResult struct {
	Stdout        string
	Stderr        string
	ChangeSummary map[apitype.OpType]int
}

type StackUpdateOptions struct {
	Message           string
	ExpectNoChanges   bool
	Diff              bool
	Replace           []string
	Target            []string
	PolicyPacks       []string
	PolicyPackConfigs []string
	TargetDependents  bool
	Parallel          int
	UserAgent         string
	Color             string
	Plan              string
	Client            string
	ExecKind          string

	Streams StackStreams
	Remote  *StackRemoteOptions
}

type StackUpdateResult struct {
	Stdout  string
	Stderr  string
	Outputs map[string]OutputValue
	Summary UpdateSummary
}

type StackRefreshOptions struct {
	Message         string
	ExpectNoChanges bool
	Target          []string
	Parallel        int
	UserAgent       string
	Color           string
	ExecKind        string

	Streams StackStreams
	Remote  *StackRemoteOptions
}

type StackRefreshResult struct {
	Stdout  string
	Stderr  string
	Summary UpdateSummary
}

type StackDestroyOptions struct {
	Message   string
	Target    []string
	Parallel  int
	UserAgent string
	Color     string
	ExecKind  string

	Streams StackStreams
	Remote  *StackRemoteOptions
}

type StackDestroyResult struct {
	Stdout  string
	Stderr  string
	Summary UpdateSummary
}

type StackInitOptions struct {
	SecretsProvider string
}

type StackRmOptions struct {
	Force bool
}

type StackSummary struct {
	Name             string `json:"name"`
	Current          bool   `json:"current"`
	LastUpdate       string `json:"lastUpdate,omitempty"`
	UpdateInProgress bool   `json:"updateInProgress"`
	ResourceCount    *int   `json:"resourceCount,omitempty"`
	URL              string `json:"url,omitempty"`
}

type OutputValue struct {
	Value  any
	Secret bool
}

type StackHistoryOptions struct {
	PageSize    int
	Page        int
	ShowSecrets bool
}

type UpdateSummary struct {
	Version     int                    `json:"version"`
	Kind        string                 `json:"kind"`
	StartTime   string                 `json:"startTime"`
	Message     string                 `json:"message"`
	Environment map[string]string      `json:"environment"`
	Config      map[string]ConfigValue `json:"config"`
	Result      string                 `json:"result,omitempty"`

	// These values are only present once the update finishes
	EndTime         *string         `json:"endTime,omitempty"`
	ResourceChanges *map[string]int `json:"resourceChanges,omitempty"`
}

type ConfigGetOptions struct {
	Path bool
}

type ConfigValue struct {
	Value  string
	Secret bool
}

type ConfigSetOptions struct {
	Path      string
	Plaintext bool
	Secret    bool
}

type ConfigRmOptions struct {
	Path bool
}
