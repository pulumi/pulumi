// Copyright 2017 Pulumi, Inc. All rights reserved.

package cloudwatch

import (
	"github.com/pulumi/coconut/pkg/resource/idl"

	"github.com/pulumi/coconut/lib/aws/idl/sns"
)

// Alarm is a CloudWatch alarm.  For more information, see
// http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cw-alarm.html.
type Alarm struct {
	idl.NamedResource
	// The arithmetic operator to use when comparing the specified statistic and threshold.  The specified statistic
	// value is used as the first operand (so, "<statistic> <op> <threshold>").
	ComparisonOperator AlarmComparisonOperator `coco:"comparisonOperator"`
	// The number of periods over which data is compared to the specific threshold.
	EvaluationPeriods float64 `coco:"evaluationPerids"`
	// The name for the alarm's associated metric.
	MetricName string `coco:"metricName"`
	// The namespace for the alarm's associated metric.
	Namespace string `coco:"namespace"`
	// The time over which the specified statistic is applied; it is a time in second that is a multiple of 60.
	Period float64 `coco:"period"`
	// The statistic to apply to the alarm's associated metric.
	Statistic AlarmStatistic `coco:"statistic"`
	// The value against which the specified statistic is compared.
	Threshold float64 `coco:"threshold"`
	// Indicates whether or not actions should be executed during any changes to the alarm's state.
	ActionsEnabled *bool `coco:"actionsEnabled,optional"`
	// The list of actions to execute hen this alarm transitions into an ALARM state from any other state.  Each action
	// is specified as an Amazon Resource Number (ARN).
	AlarmActions *[]ActionTarget `coco:"alarmActions,optional"`
	// The description for the alarm.
	AlarmDescription *string `coco:"alarmDescription,optional"`
	// A name for the alarm.  If you don't specify one, an auto-generated physical ID will be assigned.
	AlarmName *string `coco:"alarmName,replaces,optional"`
	// The dimension for the alarm's associated metric.
	Dimensions *[]AlarmDimension `coco:"dimensions,optional"`
	// The list of actions to execute when this alarm transitions into an INSUFFICIENT_DATA state from any other state.
	// Each action is specified as an Amazon Resource Number (ARN).  Currently the only action supported is publishing
	// to an Amazon SNS topic or an Amazon Auto Scaling policy.
	InsufficientDataActions *[]ActionTarget `coco:"insufficientDataActions,optional"`
	// The list of actions to execute when this alarm transitions into an OK state from any other state.  Each action is
	// specified as an Amazon Resource Number (ARN).  Currently the only action supported is publishing to an Amazon SNS
	// topic of an Amazon Auto Scaling policy.
	OKActions *[]ActionTarget `coco:"okActions,optional"`
	// The unit for the alarm's associated metric.
	Unit *AlarmMetric `coco:"unit,optional"`
}

// ActionTarget is a strongly typed capability for an action target to avoid string-based ARNs.
// TODO[pulumi/coconut#90]: once we support more resource types, we need to support Auto Scaling policies, etc.  It's
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
	Name string `coco:"name"` // the name of the dimension, from 1-255 characters in length.
	// TODO[pulumi/coconut#90]: strongly type this.
	Value interface{} `coco:"value"` // the value representing the dimension measurement, from 1-255 characters in length.
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
