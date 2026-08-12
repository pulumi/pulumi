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

// A hands-off multi-region rollout as a pulumi.Workflow: a Lambda-backed service is deployed in
// waves of increasing size, and a release only advances to the next wave after it has baked and
// every CloudWatch error alarm from the wave it just landed in is healthy.
//
//	pulumi config set version v2   # a new version admits a new release cursor at wave1
//
// Each `pulumi up` reconciles occupied waves and polls the gates once. A release that has not
// baked long enough, or whose alarms are firing, parks where it is until a later up.
package main

import (
	"context"
	"fmt"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v7/go/aws/lambda"
	awscloudwatch "github.com/pulumi/pulumi-aws/sdk/v7/go/aws/cloudwatch"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

// waves is the rollout order: each wave is one workflow node whose program deploys the service to
// every region in the wave. Later waves are bigger — an error caught early blocks the blast radius.
var waves = [][]string{
	{"us-west-2"},
	{"us-east-1", "eu-west-1"},
	{"eu-central-1", "ap-southeast-2", "us-east-2"},
}

const handlerCode = `import os
def handler(event, context):
    return {"statusCode": 200,
            "body": "%s %s from %s" % (os.environ["SERVICE"], os.environ["VERSION"], os.environ["AWS_REGION"])}
`

// deployWave is the node program for one wave: a mini Pulumi program that deploys the service to
// each of the wave's regions with an explicit per-region provider. Its exports (the wave's alarm
// names and URLs) flow into the release cursor's data, where the edge gates read them.
func deployWave(wave int, regions []string) pulumi.RunFunc {
	return func(nctx *pulumi.Context) error {
		version := config.New(nctx, "workflow").Require("version")

		providers := make(map[string]*aws.Provider, len(regions))
		for _, region := range regions {
			provider, err := aws.NewProvider(nctx, "aws-"+region, &aws.ProviderArgs{
				Region: pulumi.String(region),
			})
			if err != nil {
				return err
			}
			providers[region] = provider
		}

		// IAM is global; one execution role per wave, created via the wave's first region.
		first := pulumi.Provider(providers[regions[0]])
		role, err := iam.NewRole(nctx, fmt.Sprintf("wave%d-exec", wave), &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {"Service": "lambda.amazonaws.com"},
					"Action": "sts:AssumeRole"
				}]
			}`),
		}, first)
		if err != nil {
			return err
		}
		attach, err := iam.NewRolePolicyAttachment(nctx, fmt.Sprintf("wave%d-exec-logs", wave), &iam.RolePolicyAttachmentArgs{
			Role:      role.Name,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
		}, first)
		if err != nil {
			return err
		}

		var alarms, urls pulumi.Array
		for _, region := range regions {
			opt := pulumi.Provider(providers[region])

			fn, err := lambda.NewFunction(nctx, "svc-"+region, &lambda.FunctionArgs{
				Runtime: pulumi.String("python3.12"),
				Handler: pulumi.String("index.handler"),
				Role:    role.Arn,
				Code: pulumi.NewAssetArchive(map[string]interface{}{
					"index.py": pulumi.NewStringAsset(handlerCode),
				}),
				Environment: &lambda.FunctionEnvironmentArgs{
					Variables: pulumi.StringMap{
						"SERVICE": pulumi.String("wf-demo"),
						"VERSION": pulumi.String(version),
					},
				},
			}, opt, pulumi.DependsOn([]pulumi.Resource{attach}))
			if err != nil {
				return err
			}

			// The gate that lets a release leave this wave reads this alarm: errors in any of the
			// wave's regions hold the whole release on the conveyor.
			alarm, err := awscloudwatch.NewMetricAlarm(nctx, "errors-"+region, &awscloudwatch.MetricAlarmArgs{
				Namespace:          pulumi.String("AWS/Lambda"),
				MetricName:         pulumi.String("Errors"),
				Statistic:          pulumi.String("Sum"),
				Period:             pulumi.Int(60),
				EvaluationPeriods:  pulumi.Int(1),
				Threshold:          pulumi.Float64(1),
				ComparisonOperator: pulumi.String("GreaterThanOrEqualToThreshold"),
				TreatMissingData:   pulumi.String("notBreaching"),
				Dimensions:         pulumi.StringMap{"FunctionName": fn.Name},
			}, opt)
			if err != nil {
				return err
			}

			url, err := lambda.NewFunctionUrl(nctx, "url-"+region, &lambda.FunctionUrlArgs{
				FunctionName:      fn.Name,
				AuthorizationType: pulumi.String("NONE"),
			}, opt)
			if err != nil {
				return err
			}
			_, err = lambda.NewPermission(nctx, "public-"+region, &lambda.PermissionArgs{
				Action:              pulumi.String("lambda:InvokeFunctionUrl"),
				Function:            fn.Name,
				Principal:           pulumi.String("*"),
				FunctionUrlAuthType: pulumi.String("NONE"),
			}, opt)
			if err != nil {
				return err
			}

			alarms = append(alarms, pulumi.Map{
				"region": pulumi.String(region),
				"name":   alarm.Name,
			})
			urls = append(urls, url.FunctionUrl)
		}

		// Cursor data is structured: these exports arrive in the edge gates as a real list of
		// objects, not a string encoding.
		nctx.Export("alarms", alarms)
		nctx.Export("urls", urls)
		nctx.Export("version", pulumi.String(version))
		return nil
	}
}

// wavesHealthy is the edge gate: the release has baked in its current wave, and none of that
// wave's error alarms are firing. It runs in the workflow's own program, so it is plain Go —
// here, direct CloudWatch API calls.
func wavesHealthy(bake time.Duration) pulumi.WorkflowCondition {
	return func(cctx context.Context) (bool, error) {
		t := pulumi.WorkflowFrom(cctx)
		if time.Since(t.When) < bake {
			return false, nil
		}
		alarms, _ := t.Data()["alarms"].([]any)
		if len(alarms) == 0 {
			return false, nil // The wave has not been reconciled yet; its exports are not in the data.
		}
		for _, entry := range alarms {
			m, _ := entry.(map[string]any)
			region, _ := m["region"].(string)
			name, _ := m["name"].(string)
			if region == "" || name == "" {
				return false, fmt.Errorf("malformed alarm entry %v", entry)
			}
			cfg, err := awsconfig.LoadDefaultConfig(cctx, awsconfig.WithRegion(region))
			if err != nil {
				return false, err
			}
			out, err := cloudwatch.NewFromConfig(cfg).DescribeAlarms(cctx, &cloudwatch.DescribeAlarmsInput{
				AlarmNames: []string{name},
			})
			if err != nil {
				return false, err
			}
			for _, a := range out.MetricAlarms {
				if a.StateValue == cwtypes.StateValueAlarm {
					return false, nil
				}
			}
		}
		return true, nil
	}
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		version := cfg.Get("version")
		if version == "" {
			version = "v1"
		}
		bake := 120 * time.Second
		if s := cfg.GetInt("bakeSeconds"); s > 0 {
			bake = time.Duration(s) * time.Second
		}

		_, err := pulumi.NewWorkflow(ctx, "rollout", func(g *pulumi.WorkflowGraph) error {
			nodes := make([]*pulumi.WorkflowNode, len(waves))
			for i, regions := range waves {
				nodes[i] = g.DefNode(fmt.Sprintf("wave%d", i+1), deployWave(i+1, regions))
			}
			for i := 0; i < len(nodes)-1; i++ {
				g.Edge(nodes[i], nodes[i+1], wavesHealthy(bake))
			}
			g.Entry(nodes[0], pulumi.Map{"version": pulumi.String(version)})
			return nil
		})
		return err
	})
}
