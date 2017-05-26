// Licensed to Pulumi Corporation ("Pulumi") under one or more
// contributor license agreements.  See the NOTICE file distributed with
// this work for additional information regarding copyright ownership.
// Pulumi licenses this file to You under the Apache License, Version 2.0
// (the "License"); you may not use this file except in compliance with
// the License.  You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package elasticbeanstalk

import (
	"crypto/sha1"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awselasticbeanstalk "github.com/aws/aws-sdk-go/service/elasticbeanstalk"
	"github.com/pkg/errors"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/mapper"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/elasticbeanstalk"
)

const EnvironmentToken = elasticbeanstalk.EnvironmentToken

// constants for the various environment limits.
const (
	minCNAMEPrefix     = 4
	maxCNAMEPrefix     = 63
	minEnvironmentName = 4
	maxEnvironmentName = 63
)

// NewEnvironmentProvider creates a provider that handles ElasticBeanstalk environment operations.
func NewEnvironmentProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &environmentProvider{ctx}
	return elasticbeanstalk.NewEnvironmentProvider(ops)
}

type environmentProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *environmentProvider) Check(ctx context.Context, obj *elasticbeanstalk.Environment) ([]mapper.FieldError, error) {
	var failures []mapper.FieldError
	// TODO: Check property bag
	return failures, nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *environmentProvider) Create(ctx context.Context, obj *elasticbeanstalk.Environment) (resource.ID, *elasticbeanstalk.EnvironmentOuts, error) {
	if obj.CNAMEPrefix != nil || obj.Tags != nil || obj.TemplateName != nil || obj.Tier != nil {
		return "", nil, fmt.Errorf("Properties not yet supported: CNAMEPrefix, Tags, TemplateName, Tier")
	}
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	// TODO: use the URN, not just the name, to enhance global uniqueness.
	// TODO: even for explicit names, we should consider mangling it somehow, to reduce multi-instancing conflicts.
	var name string
	if obj.EnvironmentName != nil {
		name = *obj.EnvironmentName
	} else {
		name = resource.NewUniqueHex(obj.Name+"-", maxEnvironmentName, sha1.Size)
	}
	var optionSettings []*awselasticbeanstalk.ConfigurationOptionSetting
	if obj.OptionSettings != nil {
		for _, setting := range *obj.OptionSettings {
			optionSettings = append(optionSettings, &awselasticbeanstalk.ConfigurationOptionSetting{
				Namespace:  aws.String(setting.Namespace),
				OptionName: aws.String(setting.OptionName),
				Value:      aws.String(setting.Value),
			})
		}
	}
	fmt.Printf("Creating ElasticBeanstalk Environment '%v' with name '%v'\n", obj.Name, name)
	create := &awselasticbeanstalk.CreateEnvironmentInput{
		EnvironmentName:   aws.String(name),
		ApplicationName:   obj.Application.StringPtr(),
		Description:       obj.Description,
		OptionSettings:    optionSettings,
		VersionLabel:      obj.Version.StringPtr(),
		SolutionStackName: obj.SolutionStackName,
	}
	_, err := p.ctx.ElasticBeanstalk().CreateEnvironment(create)
	if err != nil {
		return "", nil, err
	}
	var endpointURL *string
	succ, err := awsctx.RetryUntilLong(p.ctx, func() (bool, error) {
		fmt.Printf("Waiting for environment %v to become Ready\n", name)
		resp, err := p.getEnvironment(&name)
		if err != nil {
			return false, err
		}
		if resp == nil {
			return false, fmt.Errorf("New environment was terminated before becoming ready")
		}
		if *resp.Status == "Ready" {
			endpointURL = resp.EndpointURL
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return "", nil, err
	}
	if !succ {
		return "", nil, fmt.Errorf("Timed out waiting for environment to become ready")
	}
	outs := &elasticbeanstalk.EnvironmentOuts{
		EndpointURL: *endpointURL,
	}
	fmt.Printf("Created ElasticBeanstalk Environment '%v' with EndpointURL: %v\n", name, *endpointURL)
	return resource.ID(name), outs, nil
}

// Read reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *environmentProvider) Get(ctx context.Context, id resource.ID) (*elasticbeanstalk.Environment, error) {
	return nil, errors.New("Not yet implemented")
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *environmentProvider) InspectChange(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.Environment, new *elasticbeanstalk.Environment, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *environmentProvider) Update(ctx context.Context, id resource.ID,
	old *elasticbeanstalk.Environment, new *elasticbeanstalk.Environment, diff *resource.ObjectDiff) error {
	if new.CNAMEPrefix != nil || new.Tags != nil || new.TemplateName != nil || new.Tier != nil {
		return fmt.Errorf("Properties not yet supported: CNAMEPrefix, Tags, TemplateName, Tier")
	}
	envUpdate := awselasticbeanstalk.UpdateEnvironmentInput{
		EnvironmentName: id.StringPtr(),
	}
	if diff.Changed(elasticbeanstalk.Environment_Description) {
		envUpdate.Description = new.Description
	}
	if diff.Changed(elasticbeanstalk.Environment_SolutionStackName) {
		envUpdate.SolutionStackName = new.SolutionStackName
	}
	if diff.Changed(elasticbeanstalk.Environment_OptionSettings) {
		newOptionsSet := newOptionSettingHashSet(new.OptionSettings)
		oldOptionsSet := newOptionSettingHashSet(old.OptionSettings)
		d := oldOptionsSet.Changes(newOptionsSet)
		for _, o := range d.AddOrUpdates() {
			option := o.(optionSettingHash).item
			envUpdate.OptionSettings = append(envUpdate.OptionSettings, &awselasticbeanstalk.ConfigurationOptionSetting{
				Namespace:  aws.String(option.Namespace),
				OptionName: aws.String(option.OptionName),
				Value:      aws.String(option.Value),
			})
		}
		for _, o := range d.Deletes() {
			option := o.(optionSettingHash).item
			envUpdate.OptionsToRemove = append(envUpdate.OptionsToRemove, &awselasticbeanstalk.OptionSpecification{
				Namespace:  aws.String(option.Namespace),
				OptionName: aws.String(option.OptionName),
			})
		}
	}
	if _, err := p.ctx.ElasticBeanstalk().UpdateEnvironment(&envUpdate); err != nil {
		return err
	}
	succ, err := awsctx.RetryUntilLong(p.ctx, func() (bool, error) {
		fmt.Printf("Waiting for environment %v to become Ready\n", id.String())
		resp, err := p.getEnvironment(id.StringPtr())
		if err != nil {
			return false, err
		}
		if resp == nil {
			return false, fmt.Errorf("New environment was terminated before becoming ready")
		}
		if *resp.Status == "Ready" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if !succ {
		return fmt.Errorf("Timed out waiting for environment to become ready")
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *environmentProvider) Delete(ctx context.Context, id resource.ID) error {
	fmt.Printf("Deleting ElasticBeanstalk Environment '%v'\n", id)
	if _, err := p.ctx.ElasticBeanstalk().TerminateEnvironment(&awselasticbeanstalk.TerminateEnvironmentInput{
		EnvironmentName: id.StringPtr(),
	}); err != nil {
		return err
	}
	succ, err := awsctx.RetryUntilLong(p.ctx, func() (bool, error) {
		fmt.Printf("Waiting for environment %v to become Terminated\n", id.String())
		resp, err := p.getEnvironment(id.StringPtr())
		if err != nil {
			return false, err
		}
		if resp == nil || resp.Status == nil || *resp.Status == "Terminated" {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}
	if !succ {
		return fmt.Errorf("Timed out waiting for environment to become terminated")
	}
	return nil
}

func (p *environmentProvider) getEnvironment(name *string) (*awselasticbeanstalk.EnvironmentDescription, error) {
	resp, err := p.ctx.ElasticBeanstalk().DescribeEnvironments(&awselasticbeanstalk.DescribeEnvironmentsInput{
		EnvironmentNames: []*string{name},
	})
	if err != nil {
		return nil, err
	}
	environments := resp.Environments
	if len(environments) > 1 {
		return nil, fmt.Errorf("More than one environment found with name %v", name)
	}
	if len(environments) == 0 {
		return nil, nil
	}
	environment := environments[0]
	return environment, nil
}

type optionSettingHash struct {
	item elasticbeanstalk.OptionSetting
}

var _ awsctx.Hashable = optionSettingHash{}

func (option optionSettingHash) HashKey() awsctx.Hash {
	return awsctx.Hash(option.item.Namespace + ":" + option.item.OptionName)
}
func (option optionSettingHash) HashValue() awsctx.Hash {
	return awsctx.Hash(option.item.Namespace + ":" + option.item.OptionName + ":" + option.item.Value)
}
func newOptionSettingHashSet(options *[]elasticbeanstalk.OptionSetting) *awsctx.HashSet {
	set := awsctx.NewHashSet()
	if options == nil {
		return set
	}
	for _, option := range *options {
		set.Add(optionSettingHash{option})
	}
	return set
}
