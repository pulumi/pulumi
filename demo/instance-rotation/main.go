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

// Fleet-wide AMI rotation as a pulumi.Workflow: a rotation cursor carries the target AMI through
// the fleet strictly one cluster at a time, and only advances once every instance it just
// replaced passes its EC2 status checks.
//
//	pulumi config set ami ami-xxxxxxxx   # a new AMI admits a new rotation cursor at cluster-a
//
// Each `pulumi up` reconciles the cluster the rotation currently occupies and polls its health
// gate once. Status checks take a few minutes after a replacement — the rotation parks, for
// hours or days if need be, until an up finds the cluster healthy.
package main

import (
	"context"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	awsec2 "github.com/pulumi/pulumi-aws/sdk/v7/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

const region = "us-west-2"

var clusters = []string{"cluster-a", "cluster-b", "cluster-c"}

// deployCluster is the node program for one cluster: it pins the cluster's instances to the AMI
// the rotation cursor carries. Changing the AMI replaces the instances — that is the rotation.
// The instance ids are exported into the cursor's data for the health gate.
func deployCluster(name string) pulumi.RunFunc {
	return func(nctx *pulumi.Context) error {
		ami := config.New(nctx, "workflow").Require("ami")
		instance, err := awsec2.NewInstance(nctx, name, &awsec2.InstanceArgs{
			Ami:          pulumi.String(ami),
			InstanceType: pulumi.String("t4g.nano"),
			Tags: pulumi.StringMap{
				"Name":    pulumi.String("wf-rotation-" + name),
				"Cluster": pulumi.String(name),
			},
		})
		if err != nil {
			return err
		}
		nctx.Export("instances", instance.ID())
		nctx.Export("ami", pulumi.String(ami))
		return nil
	}
}

// clusterHealthy is the edge gate: every instance the rotation just replaced reports both EC2
// status checks passing. Plain Go against the EC2 API, polled once per up, effect-free.
func clusterHealthy(cctx context.Context) (bool, error) {
	t := pulumi.WorkflowFrom(cctx)
	ids, _ := t.Data()["instances"].(string)
	if ids == "" {
		return false, nil // The cluster has not been reconciled yet; its exports are not in the data.
	}
	cfg, err := awsconfig.LoadDefaultConfig(cctx, awsconfig.WithRegion(region))
	if err != nil {
		return false, err
	}
	out, err := ec2.NewFromConfig(cfg).DescribeInstanceStatus(cctx, &ec2.DescribeInstanceStatusInput{
		InstanceIds: strings.Split(ids, ","),
	})
	if err != nil {
		return false, err
	}
	healthy := 0
	for _, s := range out.InstanceStatuses {
		if s.InstanceState != nil && s.InstanceState.Name == ec2types.InstanceStateNameRunning &&
			s.InstanceStatus != nil && s.InstanceStatus.Status == ec2types.SummaryStatusOk &&
			s.SystemStatus != nil && s.SystemStatus.Status == ec2types.SummaryStatusOk {
			healthy++
		}
	}
	// DescribeInstanceStatus omits instances that are not running, so require an affirmative
	// pass from every instance in the cluster.
	return healthy == len(strings.Split(ids, ",")), nil
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		ami := config.New(ctx, "").Require("ami")

		_, err := pulumi.NewWorkflow(ctx, "rotation", func(g *pulumi.WorkflowGraph) error {
			nodes := make([]*pulumi.WorkflowNode, len(clusters))
			for i, name := range clusters {
				nodes[i] = g.DefNode(name, deployCluster(name))
			}
			// Strictly one cluster at a time: the chain is the only path, and a cursor occupies
			// exactly one node.
			for i := 0; i < len(nodes)-1; i++ {
				g.Edge(nodes[i], nodes[i+1], clusterHealthy)
			}
			g.Entry(nodes[0], pulumi.Map{"ami": pulumi.String(ami)})
			return nil
		})
		return err
	})
}
