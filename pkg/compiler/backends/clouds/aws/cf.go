// Copyright 2016 Marapongo, Inc. All rights reserved.

package aws

import (
	"regexp"
)

// This file maps the schema for AWS CloudFormation so that we can generate it in a more typesafe way.  Please see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/template-anatomy.html for more information.  In
// general, we mark all optional properties with `json:",omitempty"`, while required ones lack `omitempty`.

var cfVersion = "2010-09-09"

// This section specifies some handy limits to improve compile-time checking.  For more details on the limits, please
// see http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/cloudformation-limits.html.
const (
	cfMaxLogicalIDChars               = 255
	cfMaxMappings                     = 100
	cfMaxMappingAttributes            = 64
	cfMaxMappingNameChars             = 255
	cfMaxOutputs                      = 60
	cfMaxOutputNameChars              = cfMaxLogicalIDChars
	cfMaxOutputDescriptionChars       = 4096
	cfMaxParameters                   = 60
	cfMaxParameterDescriptionChars    = 4000
	cfMaxParameterNameChars           = 255
	cfMaxParameterValueBytes          = 4096
	cfMaxResources                    = 200
	cfMaxResourceNameChars            = cfMaxLogicalIDChars
	cfMaxSignalWaitConditionDataBytes = 4096
	cfMaxStacks                       = 200
	cfMaxStackNameChars               = 128
	cfMaxTemplateDescriptionBytes     = 1024
	cfMaxTemplateBodySizeRequestBytes = 51200
	cfMaxTemplateBodySizeS3Bytes      = 460800
)

// cfTemplate represents a single AWS CloudFormation template.
type cfTemplate struct {
	AWSTemplateFormatVersion string       `json:",omitempty"` // template's version.
	Description              string       `json:",omitempty"` // a text string describing the template.
	Metadata                 cfMetadata   `json:",omitempty"` // objects that provide additional information.
	Parameters               cfParameters `json:",omitempty"` // values you can pass at runtime.
	Mappings                 cfMappings   `json:",omitempty"` // keys/values for conditional parameters.
	Conditions               cfConditions `json:",omitempty"` // conditions to control resource modification.
	Resources                cfResources  `json:","`          // the stack's resources and their properties.
	Outputs                  cfOutputs    `json:",omitempty"` // values returned when you view your stack.
}

var cfStackNameRegexp = regexp.MustCompile("[a-zA-Z0-9-]+")

// IsValidCFStackName checks that the given string is a valid CloudFormation stack name.  It cannot be empty, must be
// shorter than cfMaxStackNameChars, and can be comprised only of alphanumeric (a-z, A-Z, 0-9) or hyphen (-) characters.
func IsValidCFStackName(s string) bool {
	if len(s) == 0 || len(s) > cfMaxStackNameChars {
		return false
	}
	return cfStackNameRegexp.MatchString(s)
}

// cfMetadata can be used to include arbitrary JSON objects that provide details about the template.  Please see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/metadata-section-structure.html for more information.
type cfMetadata map[string]interface{}

// Some AWS CloudFormation features retrieve settings or configuration information that you define in the Metadata
// section.  You define this information in the following AWS CloudFormation-specific metadata keys:
const (
	// Defines configuration tasks for the cfn-init helper script, useful for configuring and installing applications on
	// EC2 instances; see http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-init.html.
	cfMetadataInit = "AWS::CloudFormation::Init"
	// Defines grouping and ordering of input parameters in the AWS CloudFormation console; for more information, see
	// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-cloudformation-interface.html.
	cfMetadataInterface = "AWS::CloudFormation::Interface"
	// Describes how resources are laid out in the AWS CloudFormation Designer; for more information see
	// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/working-with-templates-cfn-designer.html.
	cfMetadataDesigner = "AWS::CloudFormation::Designer"
)

// cfParameters may be used to pass values to your template when you create a stack.  With parameters, you can create
// templates that are customized each time you create a stack.  Each parameter must contain a value when you do so.
// Please see http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/parameters-section-structure.html.
type cfParameters map[string]cfParameter

type cfParameter struct {
	Type                  cfDataType    `json:","`          // the type for the parameter.
	AllowedPattern        string        `json:",omitempty"` // a regex of allowed patterns.
	AllowedValues         []interface{} `json:",omitempty"` // an array of allowed values.
	ConstraintDescription string        `json:",omitempty"` // string explaining constraint violation.
	Default               interface{}   `json:",omitempty"` // default value if no value was specified.
	Description           string        `json:",omitempty"` // a string describing the parameter.
	MaxLength             int           `json:",omitempty"` // the maximum character length for string parameters.
	MaxValue              int           `json:",omitempty"` // the maximum numeric value for number parameters.
	MinLength             int           `json:",omitempty"` // the minimum character length for string parameters.
	MinValue              int           `json:",omitempty"` // the minimum numeric value for number parameters.
	NoEcho                bool          `json:",omitempty"` // true to mask the value when someone describes the stack.
}

// cfDataType can be any one of the below enum values *or* a custom AWS type.
type cfDataType string

const (
	cfDataTypeString     cfDataType = "String"             // a literal string.
	cfDataTypeStringList            = "CommaDelimitedList" // a comma-delimited list of literal strings.
	cfDataTypeNumber                = "Number"             // an integer or float.
	cfDataTypeNumberList            = "List<Number>"       // a comma-delimited list of integers and/or floats.

	// These are AWS-specific parameter types that are built into CloudFormation.
	cfDataTypeEC2AvailabilityZone   = "AWS::EC2::AvailabilityZone::Name"
	cfDataTypeEC2AvailabilityZones  = "List<" + cfDataTypeEC2AvailabilityZone + ">"
	cfDataTypeEC2ImageID            = "AWS::EC2::Image::Id"
	cfDataTypeEC2ImageIDs           = "List<" + cfDataTypeEC2ImageID + ">"
	cfDataTypeEC2InstanceID         = "AWS::EC2::Instance::Id"
	cfDataTypeEC2InstanceIDs        = "List<" + cfDataTypeEC2InstanceID + ">"
	cfDataTypeEC2KeyName            = "AWS::EC2::KeyPair::KeyName"
	cfDataTypeEC2SecurityGroupName  = "AWS::EC2::SecurityGroup::GroupName"
	cfDataTypeEC2SecurityGroupNames = "List<" + cfDataTypeEC2SecurityGroupName + ">"
	cfDataTypeEC2SecurityGroupID    = "AWS::EC2::SecurityGroup::Id"
	cfDataTypeEC2SecurityGroupIDs   = "List<" + cfDataTypeEC2SecurityGroupID + ">"
	cfDataTypeEC2SubnetID           = "AWS::EC2::Subnet::Id"
	cfDataTypeEC2SubnetIDs          = "List<" + cfDataTypeEC2SubnetID + ">"
	cfDataTypeEC2VolumeID           = "AWS::EC2::Volume::Id"
	cfDataTypeEC2VolumeIDs          = "List<" + cfDataTypeEC2VolumeID + ">"
	cfDataTypeEC2VPCID              = "AWS::EC2::VPC::Id"
	cfDataTypeEC2VPCIDs             = "List<" + cfDataTypeEC2VPCID + ">"
	cfDataTypeRoute53ZoneID         = "AWS::Route53::HostedZone::Id"
	cfDataTypeRoute53ZoneIDs        = "List<" + cfDataTypeRoute53ZoneID + ">"
)

// cfMappings matches a key to a corresponding set of named values.  For example, if you want to set values based on a
// region, you can create a mapping that uses the region name as a key and contains the values you want to specify for
// each specific region.  The keys and values in mappings must be literal strings.  Within a mapping, each map is a key
// followed by another mapping.  Each key must be unique within the  mapping.  For more information, please see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/mappings-section-structure.html.
type cfMappings map[string]cfMapping
type cfMapping map[string]cfMappingKeyValues
type cfMappingKeyValues map[string]string

// cfConditions includes statements that define when a resource is created or when a property is defined.  Please see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/conditions-section-structure.html for more information.
// TODO: consider mapping these in a more strongly typed manner (e.g., the inner Fn::* function syntaxes).
type cfConditions map[string]interface{}

// cfLogicalID represents a resource identifier; it must match the below regexp and be unique in the template.
type cfLogicalID string

var cfLogicalIDRegexp = regexp.MustCompile("[a-zA-Z0-9]+")

// IsValidCFLogicalID checks that the given string is a valid CloudFormation logical ID.  It cannot be empty, must be
// less than cfMaxLogicalIDChars in length, and must be comprised only of alphanumeric (a-z, A-Z, 0-9) characters.
func IsValidCFLogicalID(s string) bool {
	if len(s) == 0 || len(s) > cfMaxLogicalIDChars {
		return false
	}
	return cfLogicalIDRegexp.MatchString(s)
}

// cfResources declares the AWS resources that you want as part of your stack.  It is a map of string-based logical IDs
// to structures describing the types and properties for those resources.  For more information, please see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/resources-section-structure.html.
type cfResources map[cfLogicalID]cfResource

type cfResource struct {
	Type       cfResourceType       `json:","`          // the resource type being declared.
	Properties cfResourceProperties `json:",omitempty"` // additional options for the resource.
}

// A resource type identifier always takes the form:
//     AWS::aws-product-name::data-type-name
// although custom resource types are also supported.
type cfResourceType string

// This list contains all predefined AWS resource types; note that custom resource types are also permitted; see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-template-resource-type-ref.html for more details.
const (
	cfResourceTypeAPIGatewayAccount                     cfResourceType = "AWS::ApiGateway::Account"
	cfResourceTypeAPIGatewayAPIKey                                     = "AWS::ApiGateway::ApiKey"
	cfResourceTypeAPIGatewayAuthorizer                                 = "AWS::ApiGateway::Authorizer"
	cfResourceTypeAPIGatewayBasePathMapping                            = "AWS::ApiGateway::BasePathMapping"
	cfResourceTypeAPIGatewayClientCeritificate                         = "AWS::ApiGateway::ClientCertificate"
	cfResourceTypeAPIGatewayDeployment                                 = "AWS::ApiGateway::Deployment"
	cfResourceTypeAPIGatewayMethod                                     = "AWS::ApiGateway::Method"
	cfResourceTypeAPIGatewayModel                                      = "AWS::ApiGateway::Model"
	cfResourceTypeAPIGatewayResource                                   = "AWS::ApiGateway::Resource"
	cfResourceTypeAPIGatewayRESTAPI                                    = "AWS::ApiGateway::RestApi"
	cfResourceTypeAPIGatewayStage                                      = "AWS::ApiGateway::Stage"
	cfResourceTypeAPIGatewayUsagePlan                                  = "AWS::ApiGateway::UsagePlan"
	cfResourceTypeApplicationAutoScalingTarget                         = "AWS::ApplicationAutoScaling::ScalableTarget"
	cfResourceTypeApplicationAutoScalingPolicy                         = "AWS::ApplicationAutoScaling::ScalingPolicy"
	cfResourceTypeAutoScalingGroup                                     = "AWS::AutoScaling::AutoScalingGroup"
	cfResourceTypeAutoScalingLaunchConfiguration                       = "AWS::AutoScaling::LaunchConfiguration"
	cfResourceTypeAutoScalingLifecycleHook                             = "AWS::AutoScaling::LifecycleHook"
	cfResourceTypeAutoScalingScalingPolicy                             = "AWS::AutoScaling::ScalingPolicy"
	cfResourceTypeAutoScalingScheduledAction                           = "AWS::AutoScaling::ScheduledAction"
	cfResourceTypeCertificateManagerCertificate                        = "AWS::CertificateManager::Certificate"
	cfResourceTypeCloudFormationAuthentication                         = "AWS::CloudFormation::Authentication"
	cfResourceTypeCloudFormationCustomResource                         = "AWS::CloudFormation::CustomResource"
	cfResourceTypeCloudFormationInit                                   = "AWS::CloudFormation::Init"
	cfResourceTypeCloudFormationInterface                              = "AWS::CloudFormation::Interface"
	cfResourceTypeCloudFormationStack                                  = "AWS::CloudFormation::Stack"
	cfResourceTypeCloudFormationWaitCondition                          = "AWS::CloudFormation::WaitCondition"
	cfResourceTypeCloudFormationWaitConditionHandle                    = "AWS::CloudFormation::WaitConditionHandle"
	cfResourceTypeCloudFrontDistribution                               = "AWS::CloudFront::Distribution"
	cfResourceTypeCloudTrailTrail                                      = "AWS::CloudTrail::Trail"
	cfResourceTypeCloudWatchAlarm                                      = "AWS::CloudWatch::Alarm"
	cfResourceTypeCodeCommitRepository                                 = "AWS::CodeCommit::Repository"
	cfResourceTypeCodeDeployApplication                                = "AWS::CodeDeploy::Application"
	cfResourceTypeCodeDeployDeploymentConfig                           = "AWS::CodeDeploy::DeploymentConfig"
	cfResourceTypeCodeDeployDeploymentGroup                            = "AWS::CodeDeploy::DeploymentGroup"
	cfResourceTypeCodePipelineCustomActionType                         = "AWS::CodePipeline::CustomActionType"
	cfResourceTypeCodePipelinePipeline                                 = "AWS::CodePipeline::Pipeline"
	cfResourceTypeConfigRule                                           = "AWS::Config::ConfigRule"
	cfResourceTypeConfigConfigurationRecorder                          = "AWS::Config::ConfigurationRecorder"
	cfResourceTypeConfigDeliveryChannel                                = "AWS::Config::DeliveryChannel"
	cfResourceTypeDataPipeline                                         = "AWS::DataPipeline::Pipeline"
	cfResourceTypeDirectoryServiceMicrosoftAD                          = "AWS::DirectoryService::MicrosoftAD"
	cfResourceTypeDirectoryServiceSimpleAD                             = "AWS::DirectoryService::SimpleAD"
	cfResourceTypeDynamoDBTable                                        = "AWS::DynamoDB::Table"
	cfResourceTypeEC2CustomerGateway                                   = "AWS::EC2::CustomerGateway"
	cfResourceTypeEC2DHCPOptions                                       = "AWS::EC2::DHCPOptions"
	cfResourceTypeEC2EIP                                               = "AWS::EC2::EIP"
	cfResourceTypeEC2EIPAssociation                                    = "AWS::EC2::EIPAssociation"
	cfResourceTypeEC2FlowLog                                           = "AWS::EC2::FlowLog"
	cfResourceTypeEC2Host                                              = "AWS::EC2::Host"
	cfResourceTypeEC2Instance                                          = "AWS::EC2::Instance"
	cfResourceTypeEC2InternetGateway                                   = "AWS::EC2::InternetGateway"
	cfResourceTypeEC2NATGateway                                        = "AWS::EC2::NatGateway"
	cfResourceTypeEC2NetworkACL                                        = "AWS::EC2::NetworkAcl"
	cfResourceTypeEC2NetworkACLEntry                                   = "AWS::EC2::NetworkAclEntry"
	cfResourceTypeEC2NetworkInterface                                  = "AWS::EC2::NetworkInterface"
	cfResourceTypeEC2NetworkInterfaceAttachment                        = "AWS::EC2::NetworkInterfaceAttachment"
	cfResourceTypeEC2PlacementGroup                                    = "AWS::EC2::PlacementGroup"
	cfResourceTypeEC2Route                                             = "AWS::EC2::Route"
	cfResourceTypeEC2RouteTable                                        = "AWS::EC2::RouteTable"
	cfResourceTypeEC2SecurityGroup                                     = "AWS::EC2::SecurityGroup"
	cfResourceTypeEC2SecurityGroupEgress                               = "AWS::EC2::SecurityGroupEgress"
	cfResourceTypeEC2SecurityGroupIngress                              = "AWS::EC2::SecurityGroupIngress"
	cfResourceTypeEC2SpotFleet                                         = "AWS::EC2::SpotFleet"
	cfResourceTypeEC2Subnet                                            = "AWS::EC2::Subnet"
	cfResourceTypeEC2SubnetNetworkACLAssociation                       = "AWS::EC2::SubnetNetworkAclAssociation"
	cfResourceTypeEC2SubnetRouteTableAssociation                       = "AWS::EC2::SubnetRouteTableAssociation"
	cfResourceTypeEC2Volume                                            = "AWS::EC2::Volume"
	cfResourceTypeEC2VolumeAttachment                                  = "AWS::EC2::VolumeAttachment"
	cfResourceTypeEC2VPC                                               = "AWS::EC2::VPC"
	cfResourceTypeEC2VPCDHCPOptionsAssociation                         = "AWS::EC2::VPCDHCPOptionsAssociation"
	cfResourceTypeEC2VPCEndpoint                                       = "AWS::EC2::VPCEndpoint"
	cfResourceTypeEC2VPCGatewayAttachment                              = "AWS::EC2::VPCGatewayAttachment"
	cfResourceTypeEC2VPCPeeringConnection                              = "AWS::EC2::VPCPeeringConnection"
	cfResourceTypeEC2VPNConnection                                     = "AWS::EC2::VPNConnection"
	cfResourceTypeEC2VPNConnectionRoute                                = "AWS::EC2::VPNConnectionRoute"
	cfResourceTypeEC2VPNGateway                                        = "AWS::EC2::VPNGateway"
	cfResourceTypeEC2VPNGatewayRoutePropagation                        = "AWS::EC2::VPNGatewayRoutePropagation"
	cfResourceTypeECRRepository                                        = "AWS::ECR::Repository"
	cfResourceTypeECSCluster                                           = "AWS::ECS::Cluster"
	cfResourceTypeECSService                                           = "AWS::ECS::Service"
	cfResourceTypeECSTaskDefinition                                    = "AWS::ECS::TaskDefinition"
	cfResourceTypeEFSFileSystem                                        = "AWS::EFS::FileSystem"
	cfResourceTypeEFSMountTarget                                       = "AWS::EFS::MountTarget"
	cfResourceTypeElastiCacheCacheCluster                              = "AWS::ElastiCache::CacheCluster"
	cfResourceTypeElastiCacheParameterGroup                            = "AWS::ElastiCache::ParameterGroup"
	cfResourceTypeElastiCacheReplicationGroup                          = "AWS::ElastiCache::ReplicationGroup"
	cfResourceTypeElastiCacheSecurityGroup                             = "AWS::ElastiCache::SecurityGroup"
	cfResourceTypeElastiCacheSecurityGroupIngress                      = "AWS::ElastiCache::SecurityGroupIngress"
	cfResourceTypeElastiCacheSubnetGroup                               = "AWS::ElastiCache::SubnetGroup"
	cfResourceTypeElasticBeanstalkApplication                          = "AWS::ElasticBeanstalk::Application"
	cfResourceTypeElasticBeanstalkApplicationVersion                   = "AWS::ElasticBeanstalk::ApplicationVersion"
	cfResourceTypeElasticBeanstalkConfigurationTemplate                = "AWS::ElasticBeanstalk::ConfigurationTemplate"
	cfResourceTypeElasticBeanstalkEnvironment                          = "AWS::ElasticBeanstalk::Environment"
	cfResourceTypeElasticLoadBalancingLoadBalancer                     = "AWS::ElasticLoadBalancing::LoadBalancer"
	cfResourceTypeElasticLoadBalancingV2Listener                       = "AWS::ElasticLoadBalancingV2::Listener"
	cfResourceTypeElasticLoadBalancingV2ListenerRule                   = "AWS::ElasticLoadBalancingV2::ListenerRule"
	cfResourceTypeElasticLoadBalancingV2LoadBalancer                   = "AWS::ElasticLoadBalancingV2::LoadBalancer"
	cfResourceTypeElasticLoadBalancingV2TargetGroup                    = "AWS::ElasticLoadBalancingV2::TargetGroup"
	cfResourceTypeElasticsearchDomain                                  = "AWS::Elasticsearch::Domain"
	cfResourceTypeEMRCluster                                           = "AWS::EMR::Cluster"
	cfResourceTypeEMRInstanceGroupConfig                               = "AWS::EMR::InstanceGroupConfig"
	cfResourceTypeEMRStep                                              = "AWS::EMR::Step"
	cfResourceTypeEventsRule                                           = "AWS::Events::Rule"
	cfResourceTypeGameLiftAlias                                        = "AWS::GameLift::Alias"
	cfResourceTypeGameLiftBuild                                        = "AWS::GameLift::Build"
	cfResourceTypeGameLiftFleet                                        = "AWS::GameLift::Fleet"
	cfResourceTypeIAMAccessKey                                         = "AWS::IAM::AccessKey"
	cfResourceTypeIAMGroup                                             = "AWS::IAM::Group"
	cfResourceTypeIAMInstanceProfile                                   = "AWS::IAM::InstanceProfile"
	cfResourceTypeIAMManagedPolicy                                     = "AWS::IAM::ManagedPolicy"
	cfResourceTypeIAMPolicy                                            = "AWS::IAM::Policy"
	cfResourceTypeIAMRole                                              = "AWS::IAM::Role"
	cfResourceTypeIAMUser                                              = "AWS::IAM::User"
	cfResourceTypeIAMUserToGroupAddition                               = "AWS::IAM::UserToGroupAddition"
	cfResourceTypeIoTCertificate                                       = "AWS::IoT::Certificate"
	cfResourceTypeIoTPolicy                                            = "AWS::IoT::Policy"
	cfResourceTypeIoTPolicyPrincipalAttachment                         = "AWS::IoT::PolicyPrincipalAttachment"
	cfResourceTypeIoTThing                                             = "AWS::IoT::Thing"
	cfResourceTypeIoTThingPrincipalAttachment                          = "AWS::IoT::ThingPrincipalAttachment"
	cfResourceTypeIoTTopicRule                                         = "AWS::IoT::TopicRule"
	cfResourceTypeKinesisStream                                        = "AWS::Kinesis::Stream"
	cfResourceTypeKinesisFirehoseDeliveryStream                        = "AWS::KinesisFirehose::DeliveryStream"
	cfResourceTypeKMSAlias                                             = "AWS::KMS::Alias"
	cfResourceTypeKMSKey                                               = "AWS::KMS::Key"
	cfResourceTypeLambdaEventSourceMapping                             = "AWS::Lambda::EventSourceMapping"
	cfResourceTypeLambdaAlias                                          = "AWS::Lambda::Alias"
	cfResourceTypeLambdaFunction                                       = "AWS::Lambda::Function"
	cfResourceTypeLambdaPermission                                     = "AWS::Lambda::Permission"
	cfResourceTypeLambdaVersion                                        = "AWS::Lambda::Version"
	cfResourceTypeLogsDestination                                      = "AWS::Logs::Destination"
	cfResourceTypeLogsLogGroup                                         = "AWS::Logs::LogGroup"
	cfResourceTypeLogsLogStream                                        = "AWS::Logs::LogStream"
	cfResourceTypeLogsMetricFilter                                     = "AWS::Logs::MetricFilter"
	cfResourceTypeLogsSubscriptionFilter                               = "AWS::Logs::SubscriptionFilter"
	cfResourceTypeOpsWorksApp                                          = "AWS::OpsWorks::App"
	cfResourceTypeOpsWorksElasticLoadBalancerAttachment                = "AWS::OpsWorks::ElasticLoadBalancerAttachment"
	cfResourceTypeOpsWorksInstance                                     = "AWS::OpsWorks::Instance"
	cfResourceTypeOpsWorksLayer                                        = "AWS::OpsWorks::Layer"
	cfResourceTypeOpsWorksStack                                        = "AWS::OpsWorks::Stack"
	cfResourceTypeRDSDBCluster                                         = "AWS::RDS::DBCluster"
	cfResourceTypeRDSDBClusterParameterGroup                           = "AWS::RDS::DBClusterParameterGroup"
	cfResourceTypeRDSDBInstance                                        = "AWS::RDS::DBInstance"
	cfResourceTypeRDSDBParameterGroup                                  = "AWS::RDS::DBParameterGroup"
	cfResourceTypeRDSDBSecurityGroup                                   = "AWS::RDS::DBSecurityGroup"
	cfResourceTypeRDSDBSecurityGroupIngress                            = "AWS::RDS::DBSecurityGroupIngress"
	cfResourceTypeRDSDBSubnetGroup                                     = "AWS::RDS::DBSubnetGroup"
	cfResourceTypeRDSEventSubscription                                 = "AWS::RDS::EventSubscription"
	cfResourceTypeRDSOptionGroup                                       = "AWS::RDS::OptionGroup"
	cfResourceTypeRedshiftCluster                                      = "AWS::Redshift::Cluster"
	cfResourceTypeRedshiftClusterParameterGroup                        = "AWS::Redshift::ClusterParameterGroup"
	cfResourceTypeRedshiftClusterSecurityGroup                         = "AWS::Redshift::ClusterSecurityGroup"
	cfResourceTypeRedshiftClusterSecurityGroupIngress                  = "AWS::Redshift::ClusterSecurityGroupIngress"
	cfResourceTypeRedshiftClusterSubnetGroup                           = "AWS::Redshift::ClusterSubnetGroup"
	cfResourceTypeRoute53HealhCheck                                    = "AWS::Route53::HealthCheck"
	cfResourceTypeRoute53HostedZone                                    = "AWS::Route53::HostedZone"
	cfResourceTypeRoute53RecordSet                                     = "AWS::Route53::RecordSet"
	cfResourceTypeRoute53RecordSetGroup                                = "AWS::Route53::RecordSetGroup"
	cfResourceTypeS3Bucket                                             = "AWS::S3::Bucket"
	cfResourceTypeS3Bucketpolicy                                       = "AWS::S3::BucketPolicy"
	cfResourceTypeSDBDomain                                            = "AWS::SDB::Domain"
	cfResourceTypeSNSTopic                                             = "AWS::SNS::Topic"
	cfResourceTypeSNSTopicPolicy                                       = "AWS::SNS::TopicPolicy"
	cfResourceTypeSQSQueue                                             = "AWS::SQS::Queue"
	cfResourceTypeSQSQueuePolicy                                       = "AWS::SQS::QueuePolicy"
	cfResourceTypeSSMDocument                                          = "AWS::SSM::Document"
	cfResourceTypeWAFByteMatchSet                                      = "AWS::WAF::ByteMatchSet"
	cfResourceTypeWAFIPSet                                             = "AWS::WAF::IPSet"
	cfResourceTypeWAFRule                                              = "AWS::WAF::Rule"
	cfResourceTypeWAFSizeConstraintSet                                 = "AWS::WAF::SizeConstraintSet"
	cfResourceTypeWAFSQLInjectionMatchSet                              = "AWS::WAF::SqlInjectionMatchSet"
	cfResourceTypeWAFWebACL                                            = "AWS::WAF::WebACL"
	cfResourceTypeWAFXSSMatchSet                                       = "AWS::WAF::XssMatchSet"
	cfResourceTypeWorkSpacesWorkspace                                  = "AWS::WorkSpaces::Workspace"
)

type cfResourceProperties map[string]interface{}

// cfOutputs optionally declares output values that can be imported into other stacks to create cross-stack references.
// See http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/outputs-section-structure.html for more details.
type cfOutputs map[cfLogicalID]cfOutput

type cfOutput struct {
	Description string         `json:",omitempty"` // a string that describes the output value.
	Value       interface{}    `json:","`          // the value returned in this output position.
	Export      cfOutputExport `json:",omitempty"` // the name of this output for cross-stack references.
	Condition   string         `json:",omitempty"` // a conditional name controlling when to export this output.
}

type cfOutputExport struct {
	Name interface{} `json:","` // a name for cros-stack imports, unique within regions.
}
