// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package sns

import (
	"crypto/sha1"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/sns"
	"github.com/pulumi/lumi/pkg/util/contract"
)

const TopicToken = sns.TopicToken

// constants for the various topic limits.
const (
	minTopicName             = 1
	maxTopicName             = 256
	displayNameAttributeName = "DisplayName"
)

var (
	topicNameRegexp           = regexp.MustCompile(`^[a-zA-Z0-9_\-]*$`)
	topicNameDisallowedRegexp = regexp.MustCompile(`[^a-zA-Z0-9_\-]`)
)

// NewTopicProvider creates a provider that handles SNS topic operations.
func NewTopicProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &topicProvider{ctx}
	return sns.NewTopicProvider(ops)
}

type topicProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *topicProvider) Check(ctx context.Context, obj *sns.Topic, property string) error {
	switch property {
	case sns.Topic_TopicName:
		if name := obj.TopicName; name != nil {
			if matched := topicNameRegexp.MatchString(*name); !matched {
				fmt.Printf("Failed to match regexp\n")
				return fmt.Errorf("did not match regexp %v", topicNameRegexp)
			} else if len(*name) < minTopicName {
				return fmt.Errorf("less than minimum length of %v", minTopicName)
			} else if len(*name) > maxTopicName {
				return fmt.Errorf("exceeded maximum length of %v", maxTopicName)
			}
		}
	}
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *topicProvider) Create(ctx context.Context, obj *sns.Topic) (resource.ID, error) {
	// If an explicit name is given, use it.  Otherwise, auto-generate a name in part based on the resource name.
	var name string
	if obj.TopicName != nil {
		name = *obj.TopicName
	} else {
		// SNS topic names have strict naming requirements.  To use the Name property as a prefix, we
		// need to convert it to a safe form first.
		safeName := topicNameDisallowedRegexp.ReplaceAllString(*obj.Name, "-")
		name = resource.NewUniqueHex(safeName+"-", maxTopicName, sha1.Size)
	}
	fmt.Printf("Creating SNS Topic '%v' with name '%v'\n", *obj.Name, name)
	create := &awssns.CreateTopicInput{
		Name: aws.String(name),
	}
	resp, err := p.ctx.SNS().CreateTopic(create)
	if err != nil {
		return "", err
	}
	contract.Assert(resp != nil)
	contract.Assert(resp.TopicArn != nil)
	if obj.DisplayName != nil {
		_, err := p.ctx.SNS().SetTopicAttributes(&awssns.SetTopicAttributesInput{
			TopicArn:       resp.TopicArn,
			AttributeName:  aws.String(displayNameAttributeName),
			AttributeValue: obj.DisplayName,
		})
		if err != nil {
			return "", err
		}
	}
	return resource.ID(*resp.TopicArn), nil
}

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *topicProvider) Get(ctx context.Context, id resource.ID) (*sns.Topic, error) {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return nil, err
	}
	resp, err := p.ctx.SNS().GetTopicAttributes(&awssns.GetTopicAttributesInput{
		TopicArn: aws.String(string(id)),
	})
	if err != nil {
		return nil, err
	}
	return &sns.Topic{
		TopicName:   &name,
		DisplayName: resp.Attributes[displayNameAttributeName],
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *topicProvider) InspectChange(ctx context.Context, id resource.ID,
	old *sns.Topic, new *sns.Topic, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *topicProvider) Update(ctx context.Context, id resource.ID,
	old *sns.Topic, new *sns.Topic, diff *resource.ObjectDiff) error {
	if diff.Changed(sns.Topic_DisplayName) {
		_, err := p.ctx.SNS().SetTopicAttributes(&awssns.SetTopicAttributesInput{
			TopicArn:       aws.String(string(id)),
			AttributeName:  aws.String(displayNameAttributeName),
			AttributeValue: new.DisplayName,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *topicProvider) Delete(ctx context.Context, id resource.ID) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting SNS Topic '%v'\n", name)
	_, err = p.ctx.SNS().DeleteTopic(&awssns.DeleteTopicInput{
		TopicArn: id.StringPtr(),
	})
	return err
}
