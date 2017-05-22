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

import { Bucket, Object } from "@lumi/aws/s3";
import { Application, ApplicationVersion, Environment } from "@lumi/aws/elasticbeanstalk";
import { File } from "@lumi/lumi/asset";

let sourceBucket = new Bucket("sourceBucket", {})
let source = new Object({
    bucket: sourceBucket,
    key: "testSource/app.zip",
    source: new File("app.zip")
})
let myapp = new Application("myapp", {})
let myappversion = new ApplicationVersion("myappversion", {
    application: myapp,
    sourceBundle: source
})
let myenv = new Environment("myenv", {
    application: myapp,
    version: myappversion,
    solutionStackName: "64bit Amazon Linux 2017.03 v4.1.0 running Node.js",
    optionSettings: [
        {
            namespace: "aws:autoscaling:asg",
            optionName: "MinSize",
            value: "2"
        },
        {
            namespace: "aws:autoscaling:launchconfiguration",
            optionName: "InstanceType",
            value: "t2.nano"
        }
    ]
})
