// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package sns

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/pulumi/lumi/pkg/util/contract"
	"github.com/pulumi/lumi/sdk/go/pkg/lumirpc"
	"golang.org/x/net/context"

	"github.com/pulumi/lumi/lib/aws/provider/arn"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/rpc/sns"
)

const SubscriptionToken = sns.SubscriptionToken

// NewSubscriptionProvider creates a provider that handles SNS subscription operations.
func NewSubscriptionProvider(ctx *awsctx.Context) lumirpc.ResourceProviderServer {
	ops := &subscriptionProvider{ctx}
	return sns.NewSubscriptionProvider(ops)
}

type subscriptionProvider struct {
	ctx *awsctx.Context
}

// Check validates that the given property bag is valid for a resource of the given type.
func (p *subscriptionProvider) Check(ctx context.Context, obj *sns.Subscription, property string) error {
	return nil
}

// Create allocates a new instance of the provided resource and returns its unique ID afterwards.  (The input ID
// must be blank.)  If this call fails, the resource must not have been created (i.e., it is "transacational").
func (p *subscriptionProvider) Create(ctx context.Context, obj *sns.Subscription) (resource.ID, error) {
	topicName, err := arn.ParseResourceName(obj.Topic)
	if err != nil {
		return "", err
	}
	fmt.Printf("Creating SNS Subscription on topic '%v'\n", topicName)
	create := &awssns.SubscribeInput{
		TopicArn: aws.String(string(obj.Topic)),
		Endpoint: aws.String(obj.Endpoint),
		Protocol: aws.String(string(obj.Protocol)),
	}
	resp, err := p.ctx.SNS().Subscribe(create)
	if err != nil {
		return "", err
	}
	contract.Assert(resp != nil)
	contract.Assert(resp.SubscriptionArn != nil)
	return resource.ID(*resp.SubscriptionArn), nil
}

// Query returns an (possibly non-empty) array of resource objects.
func (p *subscriptionProvider) Query(ctx context.Context) ([]*sns.SubscriptionItem, error) {
	return nil, nil
}

/*
	subs, err := p.ctx.SNS().ListSubscriptions(&awssns.ListSubscriptionsInput{})
	if err != nil {
		return nil, err
	}
	var subscriptions []*sns.Subscription
	for _, subscription := range subs.Subscriptions {
		subscriptions = append(subscriptions, &sns.Subscription{
			Topic:    resource.ID(aws.StringValue(subscription.TopicArn)),
			Endpoint: aws.StringValue(subscription.Endpoint),
			Protocol: sns.Protocol(aws.StringValue(subscription.Protocol)),
		})
	}
	return subscriptions, nil
}
*/

// Get reads the instance state identified by ID, returning a populated resource object, or an error if not found.
func (p *subscriptionProvider) Get(ctx context.Context, id resource.ID) (*sns.Subscription, error) {
	/*
		queresp, err := p.Query(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range queresp {
			if s.SubscriptionArn == id {
				return s, nil
			}
		}
		return nil, errors.New("No resource with matching ID found")
	*/
	resp, err := p.ctx.SNS().GetSubscriptionAttributes(&awssns.GetSubscriptionAttributesInput{
		SubscriptionArn: aws.String(string(id)),
	})
	if err != nil {
		return nil, err
	}
	listResp, err := p.ctx.SNS().ListSubscriptionsByTopic(&awssns.ListSubscriptionsByTopicInput{
		TopicArn: resp.Attributes["TopicArn"],
	})
	if err != nil {
		return nil, err
	}
	var subscription *awssns.Subscription
	for _, s := range listResp.Subscriptions {
		if *s.SubscriptionArn == string(id) {
			subscription = s
		}
	}
	return &sns.Subscription{
		Topic:    resource.ID(aws.StringValue(resp.Attributes["TopicArn"])),
		Endpoint: aws.StringValue(subscription.Endpoint),
		Protocol: sns.Protocol(aws.StringValue(subscription.Protocol)),
	}, nil
}

// InspectChange checks what impacts a hypothetical update will have on the resource's properties.
func (p *subscriptionProvider) InspectChange(ctx context.Context, id resource.ID,
	old *sns.Subscription, new *sns.Subscription, diff *resource.ObjectDiff) ([]string, error) {
	return nil, nil
}

// Update updates an existing resource with new values.  Only those values in the provided property bag are updated
// to new values.  The resource ID is returned and may be different if the resource had to be recreated.
func (p *subscriptionProvider) Update(ctx context.Context, id resource.ID,
	old *sns.Subscription, new *sns.Subscription, diff *resource.ObjectDiff) error {
	contract.Failf("No updatable properties on SNS Subscription")
	return nil
}

// Delete tears down an existing resource with the given ID.  If it fails, the resource is assumed to still exist.
func (p *subscriptionProvider) Delete(ctx context.Context, id resource.ID) error {
	name, err := arn.ParseResourceName(id)
	if err != nil {
		return err
	}
	fmt.Printf("Deleting SNS Subscription '%v'\n", name)
	_, err = p.ctx.SNS().Unsubscribe(&awssns.UnsubscribeInput{
		SubscriptionArn: id.StringPtr(),
	})
	return err
}
