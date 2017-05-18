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

package cloudwatch

import (
	"github.com/pulumi/lumi/pkg/resource/idl"

	"github.com/pulumi/lumi/lib/aws/idl/sns"
)

// Alarm is a CloudWatch alarm.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cw-alarm.html.
type Alarm struct {
	idl.NamedResource
	// The arithmetic operator to use when comparing the specified statistic and threshold.  The specified statistic
	// value is used as the first operand (so, "<statistic> <op> <threshold>").
	ComparisonOperator AlarmComparisonOperator `lumi:"comparisonOperator"`
	// The number of periods over which data is compared to the specific threshold.
	EvaluationPeriods float64 `lumi:"evaluationPerids"`
	// The name for the alarm's associated metric.
	MetricName string `lumi:"metricName"`
	// The namespace for the alarm's associated metric.
	Namespace string `lumi:"namespace"`
	// The time over which the specified statistic is applied; it is a time in second that is a multiple of 60.
	Period float64 `lumi:"period"`
	// The statistic to apply to the alarm's associated metric.
	Statistic AlarmStatistic `lumi:"statistic"`
	// The value against which the specified statistic is compared.
	Threshold float64 `lumi:"threshold"`
	// Indicates whether or not actions should be executed during any changes to the alarm's state.
	ActionsEnabled *bool `lumi:"actionsEnabled,optional"`
	// The list of actions to execute hen this alarm transitions into an ALARM state from any other state.  Each action
	// is specified as an Amazon Resource Number (ARN).
	AlarmActions *[]ActionTarget `lumi:"alarmActions,optional"`
	// The description for the alarm.
	AlarmDescription *string `lumi:"alarmDescription,optional"`
	// A name for the alarm.  If you don't specify one, an auto-generated physical ID will be assigned.
	AlarmName *string `lumi:"alarmName,replaces,optional"`
	// The dimension for the alarm's associated metric.
	Dimensions *[]AlarmDimension `lumi:"dimensions,optional"`
	// The list of actions to execute when this alarm transitions into an INSUFFICIENT_DATA state from any other state.
	// Each action is specified as an Amazon Resource Number (ARN).  Currently the only action supported is publishing
	// to an Amazon SNS topic or an Amazon Auto Scaling policy.
	InsufficientDataActions *[]ActionTarget `lumi:"insufficientDataActions,optional"`
	// The list of actions to execute when this alarm transitions into an OK state from any other state.  Each action is
	// specified as an Amazon Resource Number (ARN).  Currently the only action supported is publishing to an Amazon SNS
	// topic of an Amazon Auto Scaling policy.
	OKActions *[]ActionTarget `lumi:"okActions,optional"`
	// The unit for the alarm's associated metric.
	Unit *AlarmMetric `lumi:"unit,optional"`
}

// ActionTarget is a strongly typed capability for an action target to avoid string-based ARNs.
// TODO[pulumi/lumi#90]: once we support more resource types, we need to support Auto Scaling policies, etc.  It's
//     not yet clear whether we should do this using ARNs, or something else.
type ActionTarget sns.Topic

// AlarmComparisonOperator represents the operator (>=, >, <, or <=) used for alarm threshold comparisons.
type AlarmComparisonOperator string

const (
	ThresholdGreaterThanOrEqualTo AlarmComparisonOperator = "GreaterThanOrEqualToThreshold"
	ThresholdGreaterThan          AlarmComparisonOperator = "GreaterThanThreshold"
	ThresholdLessThan             AlarmComparisonOperator = "LessThanThreshold"
	ThresholdLessThanOrEqualTo    AlarmComparisonOperator = "LessThanOrEqualToThreshold"
)

// AlarmStatistic represents the legal values for an alarm's statistic.
type AlarmStatistic string

const (
	SampleCountStatistic AlarmStatistic = "SampleCount"
	AverageStatistic     AlarmStatistic = "Average"
	SumStatistic         AlarmStatistic = "Sum"
	MinimumStatistic     AlarmStatistic = "Minimum"
	MaximumStatistic     AlarmStatistic = "Maximum"
)

// AlarmDimension is an embedded property of the alarm type.  Dimensions are arbitrary name/value pairs that can be
// associated with a CloudWatch metric.  You can specify a maximum of 10 dimensions for a given metric.
type AlarmDimension struct {
	Name string `lumi:"name"` // the name of the dimension, from 1-255 characters in length.
	// TODO[pulumi/lumi#90]: strongly type this.
	Value interface{} `lumi:"value"` // the value representing the dimension measurement, from 1-255 characters in length.
}

// AlarmMetric represents the legal values for an alarm's associated metric.
type AlarmMetric string

const (
	SecondsMetric            AlarmMetric = "Seconds"
	MicrosecondsMetric       AlarmMetric = "Microseconds"
	MillisecondsMetric       AlarmMetric = "Milliseconds"
	BytesMetric              AlarmMetric = "Bytes"
	KilobytesMetric          AlarmMetric = "Kilobytes"
	MegabytesMetric          AlarmMetric = "Megabytes"
	GigabytesMetric          AlarmMetric = "Gigabytes"
	TerabytesMetric          AlarmMetric = "Terabytes"
	BytesPerSecondMetric     AlarmMetric = "Bytes/Second"
	KilobytesPerSecondMetric AlarmMetric = "Kilobytes/Second"
	MegabytesPerSecondMetric AlarmMetric = "Megabytes/Second"
	GigabytesPerSecondMetric AlarmMetric = "Gigabytes/Second"
	TerabytesPerSecondMetric AlarmMetric = "Terabytes/Second"
	BitsMetric               AlarmMetric = "Bits"
	KilobitsMetric           AlarmMetric = "Kilobits"
	MegabitsMetric           AlarmMetric = "Megabits"
	GigabitsMetric           AlarmMetric = "Gigabits"
	TerabitsMetric           AlarmMetric = "Terabits"
	BitsPerSecondMetric      AlarmMetric = "Bits/Second"
	KilobitsPerSecondMetric  AlarmMetric = "Kilobits/Second"
	MegabitsPerSecondMetric  AlarmMetric = "Megabits/Second"
	GigabitsPerSecondMetric  AlarmMetric = "Gigabits/Second"
	TerabitsPerSecondMetric  AlarmMetric = "Terabits/Second"
	PercentMetric            AlarmMetric = "Percent"
	CountMetric              AlarmMetric = "Count"
	CountPerSecondMetric     AlarmMetric = "Count/Second"
	NoMetric                 AlarmMetric = "None"
)
