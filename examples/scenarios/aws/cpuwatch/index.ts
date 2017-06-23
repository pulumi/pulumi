// Copyright 2016-2017, Pulumi Corporation
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

import * as aws from "@lumi/aws";
import {emailAddress} from "./config";

// enableAlarm turns on a CPU alarm for the given instance; if the CPU utilization exceeds the given threshold, an
// email will be sent to the configured email address.
export function enableAlarm(instance: aws.ec2.Instance, threshold: number): void {
    if (!emailAddress) {
        throw new Error("Missing email address configuration");
    }

    let topic = new aws.sns.Topic(instance.name + "-topic", {
        subscription: [{
            protocol: "email",
            endpoint: emailAddress,
        }],
    });
    let alarm = new aws.cloudwatch.Alarm(instance.name + "-alarm", {
        alarmActions: [ topic ],
        metricName: "CPUUtilization",
        namespace: "AWS/EC2",
        statistic: "Average",
        period: 60,
        evaluationPeriods: 3,
        threshold: threshold,
        comparisonOperator: "GreaterThanThreshold",
        dimensions: [{
            name: "InstanceId",
            value: instance,
        }],
    });
}

