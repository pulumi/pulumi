// Copyright 2016-2018, Pulumi Corporation.
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

package testing

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pulumi/pulumi/pkg/v2/backend"
	"github.com/pulumi/pulumi/pkg/v2/engine"
	"github.com/pulumi/pulumi/sdk/v2/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v2/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v2/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v2/go/common/workspace"
)

var _ backend.Client = (*Client)(nil)

type Stack struct {
	m sync.RWMutex

	ID          backend.StackIdentifier
	Tags        map[string]string
	Updates     []apitype.UpdateInfo
	Checkpoints []apitype.DeploymentV3

	update *Update
}

func (s *Stack) Marshal() apitype.Stack {
	tags := map[apitype.StackTagName]string{}
	for k, v := range s.Tags {
		tags[k] = v
	}

	activeUpdate := ""
	if len(s.Updates) != 0 {
		activeUpdate = fmt.Sprintf("%v", len(s.Updates))
	}

	return apitype.Stack{
		OrgName:      s.ID.Owner,
		ProjectName:  s.ID.Project,
		StackName:    tokens.QName(s.ID.Stack),
		ActiveUpdate: activeUpdate,
		Tags:         tags,
		Version:      len(s.Updates),
	}
}

type ClientConfig struct {
	Name                  string
	URL                   string
	User                  string
	DefaultSecretsManager string
}

type Client struct {
	m sync.RWMutex

	Config ClientConfig
	Stacks map[backend.StackIdentifier]*Stack
}

func NewClient(config ClientConfig, stacks ...*Stack) *Client {
	stackMap := map[backend.StackIdentifier]*Stack{}
	for _, s := range stacks {
		stackMap[s.ID] = s
	}
	return &Client{Config: config, Stacks: stackMap}
}

func (c *Client) Name() string {
	return c.Config.Name
}

func (c *Client) URL() string {
	return c.Config.URL
}

func (c *Client) User(ctx context.Context) (string, error) {
	return c.Config.User, nil
}

func (c *Client) DefaultSecretsManager() string {
	return c.Config.DefaultSecretsManager
}

func (c *Client) DoesProjectExist(ctx context.Context, owner, projectName string) (bool, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	for id := range c.Stacks {
		if id.Owner == owner && id.Project == projectName {
			return true, nil
		}
	}
	return false, nil
}

func (c *Client) StackConsoleURL(stackID backend.StackIdentifier) (string, error) {
	return "", nil
}

func (c *Client) ListStacks(ctx context.Context, filter backend.ListStacksFilter) ([]apitype.StackSummary, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	accept := func(id backend.StackIdentifier, stack *Stack) bool {
		if filter.Organization != nil && id.Owner != *filter.Organization {
			return false
		}
		if filter.Project != nil && id.Project != *filter.Project {
			return false
		}
		if filter.TagName != nil && filter.TagValue != nil && stack.Tags[*filter.TagName] != *filter.TagValue {
			return false
		}
		return true
	}

	var summaries []apitype.StackSummary
	for id, stack := range c.Stacks {
		if accept(id, stack) {
			var lastUpdate *int64
			if len(stack.Updates) != 0 {
				lastUpdate = &stack.Updates[len(stack.Updates)-1].EndTime
			}
			var resourceCount *int
			if len(stack.Checkpoints) != 0 {
				count := len(stack.Checkpoints[len(stack.Checkpoints)-1].Resources)
				resourceCount = &count
			}

			summaries = append(summaries, apitype.StackSummary{
				OrgName:       id.Owner,
				ProjectName:   id.Project,
				StackName:     id.Stack,
				LastUpdate:    lastUpdate,
				ResourceCount: resourceCount,
			})
		}
	}
	return summaries, nil
}

func (c *Client) GetStack(ctx context.Context, stackID backend.StackIdentifier) (apitype.Stack, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	stack, ok := c.Stacks[stackID]
	if !ok {
		return apitype.Stack{}, backend.ErrNotFound
	}
	return stack.Marshal(), nil
}

func (c *Client) CreateStack(ctx context.Context, stackID backend.StackIdentifier,
	tags map[string]string) (apitype.Stack, error) {

	c.m.Lock()
	defer c.m.Unlock()

	if _, ok := c.Stacks[stackID]; ok {
		return apitype.Stack{}, fmt.Errorf("stack %v already exists", stackID)
	}

	s := &Stack{
		ID:   stackID,
		Tags: tags,
	}
	if c.Stacks == nil {
		c.Stacks = map[backend.StackIdentifier]*Stack{}
	}
	c.Stacks[stackID] = s
	return s.Marshal(), nil
}

func (c *Client) DeleteStack(ctx context.Context, stackID backend.StackIdentifier, force bool) (bool, error) {
	c.m.Lock()
	defer c.m.Unlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return false, backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	if s.update != nil {
		return false, fmt.Errorf("stack is currently being updated")
	}

	if len(s.Checkpoints) != 0 && len(s.Checkpoints[len(s.Checkpoints)-1].Resources) != 0 && !force {
		return false, fmt.Errorf("stack still contains resources")
	}

	delete(c.Stacks, stackID)
	return true, nil
}

func (c *Client) RenameStack(ctx context.Context, currentID, newID backend.StackIdentifier) error {
	c.m.Lock()
	defer c.m.Unlock()

	s, ok := c.Stacks[currentID]
	if !ok {
		return backend.ErrNotFound
	}
	if _, ok := c.Stacks[newID]; ok {
		return fmt.Errorf("stack %v already exists", newID)
	}

	s.ID = newID
	c.Stacks[newID] = s
	delete(c.Stacks, currentID)
	return nil
}

func (c *Client) UpdateStackTags(ctx context.Context, stackID backend.StackIdentifier, tags map[string]string) error {
	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	s.Tags = tags
	return nil
}

func (c *Client) GetStackHistory(ctx context.Context, stackID backend.StackIdentifier) ([]apitype.UpdateInfo, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return nil, backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	return s.Updates, nil
}

func (c *Client) GetLatestStackConfig(ctx context.Context, stackID backend.StackIdentifier) (config.Map, error) {
	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return nil, backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	if len(s.Updates) == 0 {
		return nil, backend.ErrNoPreviousDeployment
	}

	cfg := config.Map{}
	for k, v := range s.Updates[len(s.Updates)-1].Config {
		newKey, err := config.ParseKey(k)
		if err != nil {
			return nil, err
		}
		if v.Object {
			if v.Secret {
				cfg[newKey] = config.NewSecureObjectValue(v.String)
			} else {
				cfg[newKey] = config.NewObjectValue(v.String)
			}
		} else {
			if v.Secret {
				cfg[newKey] = config.NewSecureValue(v.String)
			} else {
				cfg[newKey] = config.NewValue(v.String)
			}
		}
	}
	return cfg, nil
}

func (c *Client) ExportStackDeployment(ctx context.Context, stackID backend.StackIdentifier,
	version *int) (apitype.UntypedDeployment, error) {

	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return apitype.UntypedDeployment{}, backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	var deployment apitype.DeploymentV3
	if len(s.Checkpoints) != 0 {
		deployment = s.Checkpoints[len(s.Checkpoints)-1]
	}

	bytes, err := json.Marshal(deployment)
	if err != nil {
		return apitype.UntypedDeployment{}, err
	}
	return apitype.UntypedDeployment{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Deployment: bytes,
	}, nil
}

func (c *Client) ImportStackDeployment(ctx context.Context, stackID backend.StackIdentifier,
	deployment *apitype.UntypedDeployment) error {

	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return backend.ErrNotFound
	}

	if deployment.Version != apitype.DeploymentSchemaVersionCurrent {
		return fmt.Errorf("only deployment version %v is supported", apitype.DeploymentSchemaVersionCurrent)
	}

	var deploymentV3 apitype.DeploymentV3
	if err := json.Unmarshal(deployment.Deployment, &deploymentV3); err != nil {
		return err
	}

	s.m.Lock()
	defer s.m.Unlock()

	s.Updates = append(s.Updates, apitype.UpdateInfo{
		Kind:      apitype.StackImportUpdate,
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Unix(),
	})
	s.Checkpoints = append(s.Checkpoints, deploymentV3)
	return nil
}

func (c *Client) StartUpdate(ctx context.Context, kind apitype.UpdateKind, stackID backend.StackIdentifier,
	proj *workspace.Project, cfg config.Map, metadata apitype.UpdateMetadata, opts engine.UpdateOptions,
	tags map[string]string, dryRun bool) (backend.Update, error) {

	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return nil, backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	if s.update != nil {
		return nil, backend.ConflictingUpdateError{Err: fmt.Errorf("an update is already in progress")}
	}

	wireConfig := make(map[string]apitype.ConfigValue)
	for k, cv := range cfg {
		v, err := cv.Value(config.NopDecrypter)
		contract.AssertNoError(err)

		wireConfig[k.String()] = apitype.ConfigValue{
			String: v,
			Secret: cv.Secure(),
			Object: cv.Object(),
		}
	}

	s.update = &Update{s: s}
	if !dryRun {
		s.Updates = append(s.Updates, apitype.UpdateInfo{
			Kind:        kind,
			StartTime:   time.Now().Unix(),
			Message:     metadata.Message,
			Environment: metadata.Environment,
			Config:      wireConfig,
			Version:     len(s.Updates) + 1,
		})
		s.Checkpoints = append(s.Checkpoints, apitype.DeploymentV3{})

		s.update.info = &s.Updates[len(s.Updates)-1]
		s.update.checkpoint = &s.Checkpoints[len(s.Checkpoints)-1]
	} else {
		var info apitype.UpdateInfo
		var checkpoint apitype.DeploymentV3

		s.update.info = &info
		s.update.checkpoint = &checkpoint
	}

	return s.update, nil
}

func (c *Client) CancelCurrentUpdate(ctx context.Context, stackID backend.StackIdentifier) error {
	c.m.RLock()
	defer c.m.RUnlock()

	s, ok := c.Stacks[stackID]
	if !ok {
		return backend.ErrNotFound
	}

	s.m.Lock()
	defer s.m.Unlock()

	s.update = nil
	return nil
}

type Update struct {
	s          *Stack
	info       *apitype.UpdateInfo
	checkpoint *apitype.DeploymentV3
}

func (u *Update) ProgressURL() string {
	return ""
}

func (u *Update) PermalinkURL() string {
	return ""
}

func (u *Update) RequiredPolicies() []apitype.RequiredPolicy {
	return nil
}

func (u *Update) RecordEvent(ctx context.Context, event apitype.EngineEvent) error {
	if event.SummaryEvent != nil {
		u.s.m.Lock()
		defer u.s.m.Unlock()

		if u.s.update != u {
			return fmt.Errorf("update cancelled")
		}

		changes := map[apitype.OpType]int{}
		for k, v := range event.SummaryEvent.ResourceChanges {
			changes[apitype.OpType(k)] = v
		}
		u.info.ResourceChanges = changes
	}
	return nil
}

func (u *Update) PatchCheckpoint(ctx context.Context, deployment *apitype.DeploymentV3) error {
	u.s.m.Lock()
	defer u.s.m.Unlock()

	if u.s.update != u {
		return fmt.Errorf("update cancelled")
	}

	*u.checkpoint = *deployment
	return nil
}

func (u *Update) Complete(ctx context.Context, status apitype.UpdateStatus) error {
	u.s.m.Lock()
	defer u.s.m.Unlock()

	if u.s.update != u {
		return fmt.Errorf("update cancelled")
	}

	if status == apitype.StatusSucceeded {
		u.info.Result = apitype.SucceededResult
	} else {
		u.info.Result = apitype.FailedResult
	}

	u.info.EndTime = time.Now().Unix()

	u.s.update = nil
	return nil
}
