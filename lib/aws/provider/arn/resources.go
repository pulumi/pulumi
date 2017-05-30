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

package arn

import (
	"github.com/pulumi/lumi/pkg/resource"
)

// This file contains handy factories for creating all sorts of AWS resource ARNs.  In the fullness of time, it should
// contain all of those listed at http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html.  We are
// implementing them "as we go", however, so please feel free to add any that you need and which are presently missing.

func NewEC2Instance(region, accountID, id string) ARN {
	return NewResource("ec2", region, accountID, "instance", id)
}

func NewEC2InstanceID(region, accountID, id string) resource.ID {
	return resource.ID(NewEC2Instance(region, accountID, id))
}

func NewEC2SecurityGroup(region, accountID, id string) ARN {
	return NewResource("ec2", region, accountID, "security-group", id)
}

func NewEC2SecurityGroupID(region, accountID, id string) resource.ID {
	return resource.ID(NewEC2SecurityGroup(region, accountID, id))
}

func NewEC2VPC(region, accountID, id string) ARN {
	return NewResource("ec2", region, accountID, "vpc", id)
}

func NewEC2VPCID(region, accountID, id string) resource.ID {
	return resource.ID(NewEC2VPC(region, accountID, id))
}

func NewElasticBeanstalkApplication(region, accountID, name string) ARN {
	return NewResourceAlt("elasticbeanstalk", region, accountID, "application", name)
}

func NewElasticBeanstalkApplicationID(region, accountID, name string) resource.ID {
	return resource.ID(NewElasticBeanstalkApplication(region, accountID, name))
}

func NewElasticBeanstalkApplicationVersion(region, accountID, appname, versionlabel string) ARN {
	return NewResourceAlt("elasticbeanstalk", region, accountID,
		"applicationversion", appname+arnPathDelimiter+versionlabel)
}

func NewElasticBeanstalkApplicationVersionID(region, accountID, appname, versionlabel string) resource.ID {
	return resource.ID(NewElasticBeanstalkApplicationVersion(region, accountID, appname, versionlabel))
}

func NewElasticBeanstalkEnvironment(region, accountID, appname, envname string) ARN {
	return NewResourceAlt("elasticbeanstalk", region, accountID,
		"applicationversion", appname+arnPathDelimiter+envname)
}

func NewElasticBeanstalkEnvironmentID(region, accountID, appname, envname string) resource.ID {
	return resource.ID(NewElasticBeanstalkApplicationVersion(region, accountID, appname, envname))
}

func NewS3Bucket(bucket string) ARN {
	return NewResource("s3", "", "", "", bucket)
}

func NewS3BucketID(bucket string) resource.ID {
	return resource.ID(NewS3Bucket(bucket))
}

func NewS3Object(bucket, key string) ARN {
	return NewResource("s3", "", "", "", bucket+arnPathDelimiter+key)
}

func NewS3ObjectID(bucket, key string) resource.ID {
	return resource.ID(NewS3Object(bucket, key))
}
