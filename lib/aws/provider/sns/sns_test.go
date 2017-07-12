// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package sns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	awssns "github.com/aws/aws-sdk-go/service/sns"
	"github.com/pulumi/lumi/lib/aws/provider/awsctx"
	"github.com/pulumi/lumi/lib/aws/provider/testutil"
	"github.com/pulumi/lumi/lib/aws/rpc/sns"
	"github.com/pulumi/lumi/pkg/resource"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	t.Parallel()

	prefix := resource.NewUniqueHex("lumitest", 20, 20)
	ctx := testutil.CreateContext(t)
	defer func() {
		err := cleanupTopics(prefix, ctx)
		assert.Nil(t, err)
	}()

	resources := map[string]testutil.Resource{
		"topic": {Provider: NewTopicProvider(ctx), Token: TopicToken},
	}
	steps := []testutil.Step{
		{
			testutil.ResourceGenerator{
				Name: "topic",
				Creator: func(ctx testutil.Context) interface{} {
					return &sns.Topic{
						Name:        aws.String(prefix),
						DisplayName: aws.String(prefix),
					}
				},
			},
		},
	}

	props := testutil.ProviderTest(t, resources, steps)
	assert.NotNil(t, props)
}

func cleanupTopics(prefix string, ctx *awsctx.Context) error {
	fmt.Printf("Cleaning up topic with name:%v\n", prefix)
	list, err := ctx.SNS().ListTopics(&awssns.ListTopicsInput{})
	if err != nil {
		return err
	}
	cleaned := 0
	for _, topic := range list.Topics {
		if strings.Contains(aws.StringValue(topic.TopicArn), prefix) {
			if _, delerr := ctx.SNS().DeleteTopic(&awssns.DeleteTopicInput{
				TopicArn: topic.TopicArn,
			}); delerr != nil {
				fmt.Printf("Unable to cleanup topic %v: %v\n", topic.TopicArn, delerr)
				return delerr
			}
			cleaned++
		}
	}
	fmt.Printf("Cleaned up %v topics\n", cleaned)
	return nil
}
