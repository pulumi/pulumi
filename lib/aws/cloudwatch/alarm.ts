// Copyright 2017 Pulumi, Inc. All rights reserved.

import * as cloudformation from "../cloudformation";
import * as sns from "../sns";

// A CloudWatch alarm.
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cw-alarm.html
export class Alarm
        extends cloudformation.Resource
        implements AlarmProperties {
    public comparisonOperator: AlarmComparisonOperator;
    public evaluationPeriods: number;
    public metricName: string;
    public namespace: string;
    public period: number;
    public statistic: AlarmStatistic;
    public threshold: number;
    public actionsEnabled?: boolean;
    public alarmActions?: ActionTarget[];
    public alarmDescription?: string;
    public readonly alarmName?: string;
    public dimensions?: AlarmDimension[];
    public insufficientDataActions?: ActionTarget[];
    public okActions?: ActionTarget[];
    public unit?: AlarmMetric;

    constructor(name: string, args: AlarmProperties) {
        super({
            name: name,
            resource:  "AWS::CloudWatch::Alarm",
        });
        this.comparisonOperator = args.comparisonOperator;
        this.evaluationPeriods = args.evaluationPeriods;
        this.metricName = args.metricName;
        this.namespace = args.namespace;
        this.period = args.period;
        this.statistic = args.statistic;
        this.threshold = args.threshold;
        this.actionsEnabled = args.actionsEnabled;
        this.alarmActions = args.alarmActions;
        this.alarmDescription = args.alarmDescription;
        this.alarmName = args.alarmName;
        this.dimensions = args.dimensions;
        this.insufficientDataActions = args.insufficientDataActions;
        this.okActions = args.okActions;
        this.unit = args.unit;
    }
}

export interface AlarmProperties {
    // The arithmetic operator to use when comparing the specified statistic and threshold.  The specified statistic
    // value is used as the first operand (so, "<statistic> <op> <threshold>").
    comparisonOperator: AlarmComparisonOperator;
    // The number of periods over which data is compared to the specific threshold.
    evaluationPeriods: number;
    // The name for the alarm's associated metric.
    metricName: string;
    // The namespace for the alarm's associated metric.
    namespace: string;
    // The time over which the specified statistic is applied; it is a time in second that is a multiple of 60.
    period: number;
    // The statistic to apply to the alarm's associated metric.
    statistic: AlarmStatistic;
    // The value against which the specified statistic is compared.
    threshold: number;
    // Indicates whether or not actions should be executed during any changes to the alarm's state.
    actionsEnabled?: boolean;
    // The list of actions to execute hen this alarm transitions into an ALARM state from any other state.  Each action
    // is specified as an Amazon Resource Number (ARN).
    alarmActions?: ActionTarget[];
    // The description for the alarm.
    alarmDescription?: string;
    // A name for the alarm.  If you don't specify one, an auto-generated physical ID will be assigned.
    readonly alarmName?: string;
    // The dimension for the alarm's associated metric.
    dimensions?: AlarmDimension[];
    // The list of actions to execute when this alarm transitions into an INSUFFICIENT_DATA state from any other state.
    // Each action is specified as an Amazon Resource Number (ARN).  Currently the only action supported is publishing
    // to an Amazon SNS topic or an Amazon Auto Scaling policy.
    insufficientDataActions?: ActionTarget[];
    // The list of actions to execute when this alarm transitions into an OK state from any other state.  Each action is
    // specified as an Amazon Resource Number (ARN).  Currently the only action supported is publishing to an Amazon SNS
    // topic of an Amazon Auto Scaling policy.
    okActions?: ActionTarget[];
    // The unit for the alarm's associated metric.
    unit?: AlarmMetric;
}

// ActionTarget is a strongly typed capability for an action target to avoid string-based ARNs.
// TODO[pulumi/coconut#90]: once we support more resource types, we need to support Auto Scaling policies, etc.  It's
//     not yet clear whether we should do this using ARNs, or something else.
export type ActionTarget = sns.Topic;

// AlarmComparisonOperator represents the operator (>=, >, <, or <=) used for alarm threshold comparisons.
export type AlarmComparisonOperator =
    "GreaterThanOrEqualToThreshold" | "GreaterThanThreshold" |
    "LessThanThreshold" | "LessThanOrEqualToThreshold";

// AlarmStatistic represents the legal values for an alarm's statistic.
export type AlarmStatistic = "SampleCount" | "Average" | "Sum" | "Minimum" | "Maximum";

// AlarmDimension is an embedded property of the alarm type.  Dimensions are arbitrary name/value pairs that can be
// associated with a CloudWatch metric.  You can specify a maximum of 10 dimensions for a given metric.
export interface AlarmDimension {
    name: string; // the name of the dimension, from 1-255 characters in length.
    // TODO[pulumi/coconut#90]: strongly type this.
    value: any; // the value representing the dimension measurement, from 1-255 characters in length.
}

// AlarmMetric represents the legal values for an alarm's associated metric.
export type AlarmMetric =
    "Seconds" | "Microseconds" | "Milliseconds" |
    "Bytes" | "Kilobytes" | "Megabytes" | "Gigabytes" | "Terabytes" |
    "Bytes/Second" | "Kilobytes/Second" | "Megabytes/Second" | "Gigabytes/Second" | "Terabytes/Second" |
    "Bits" | "Kilobits" | "Megabits" | "Gigabits" | "Terabits" |
    "Bits/Second" | "Kilobits/Second" | "Megabits/Second" | "Gigabits/Second" | "Terabits/Second" |
    "Percent" | "Count" | "Cound/Second" | "None";

