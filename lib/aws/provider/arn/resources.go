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

// This file contains constants and factories for all sorts of AWS resource ARNs.  In the fullness of time, it should
// contain all of those listed at http://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html.  We are
// implementing them "as we go", however, so please feel free to add any that you need and which are presently missing.

const (
	EC2              = "ec2"
	EC2Instance      = "intance"
	EC2SecurityGroup = "security-group"
	EC2VPC           = "vpc"
)

func NewEC2Instance(region, accountID, id string) ARN {
	return NewResource(EC2, region, accountID, EC2Instance, id)
}

func NewEC2InstanceID(region, accountID, id string) resource.ID {
	return resource.ID(NewEC2Instance(region, accountID, id))
}

func NewEC2SecurityGroup(region, accountID, id string) ARN {
	return NewResource(EC2, region, accountID, EC2SecurityGroup, id)
}

func NewEC2SecurityGroupID(region, accountID, id string) resource.ID {
	return resource.ID(NewEC2SecurityGroup(region, accountID, id))
}

func NewEC2VPC(region, accountID, id string) ARN {
	return NewResource(EC2, region, accountID, EC2VPC, id)
}

func NewEC2VPCID(region, accountID, id string) resource.ID {
	return resource.ID(NewEC2VPC(region, accountID, id))
}

const (
	ElasticBeanstalk                   = "elasticbeanstalk"
	ElasticBeanstalkApplication        = "application"
	ElasticBeanstalkApplicationVersion = "applicationversion"
	ElasticBeanstalkEnvironment        = "environment"
)

func NewElasticBeanstalkApplication(region, accountID, name string) ARN {
	return NewResourceAlt(ElasticBeanstalk, region, accountID, ElasticBeanstalkApplication, name)
}

func NewElasticBeanstalkApplicationID(region, accountID, name string) resource.ID {
	return resource.ID(NewElasticBeanstalkApplication(region, accountID, name))
}

func NewElasticBeanstalkApplicationVersion(region, accountID, appname, versionlabel string) ARN {
	return NewResourceAlt(ElasticBeanstalk, region, accountID,
		ElasticBeanstalkApplicationVersion, appname+arnPathDelimiter+versionlabel)
}

func NewElasticBeanstalkApplicationVersionID(region, accountID, appname, versionlabel string) resource.ID {
	return resource.ID(NewElasticBeanstalkApplicationVersion(region, accountID, appname, versionlabel))
}

func NewElasticBeanstalkEnvironment(region, accountID, appname, envname string) ARN {
	return NewResourceAlt(ElasticBeanstalk, region, accountID,
		ElasticBeanstalkEnvironment, appname+arnPathDelimiter+envname)
}

const (
	S3 = "S3"
)

func NewElasticBeanstalkEnvironmentID(region, accountID, appname, envname string) resource.ID {
	return resource.ID(NewElasticBeanstalkApplicationVersion(region, accountID, appname, envname))
}

func NewS3Bucket(bucket string) ARN {
	return NewResource(S3, "", "", "", bucket)
}

func NewS3BucketID(bucket string) resource.ID {
	return resource.ID(NewS3Bucket(bucket))
}

func NewS3Object(bucket, key string) ARN {
	return NewResource(S3, "", "", "", bucket+arnPathDelimiter+key)
}

func NewS3ObjectID(bucket, key string) resource.ID {
	return resource.ID(NewS3Object(bucket, key))
}
