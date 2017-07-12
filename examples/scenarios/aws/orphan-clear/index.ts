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

import * as aws from "@lumi/aws";
import * as lumi from "@lumi/lumi";

let rescNames = []; // For loop filling array with GroupNames of ALL securitygroups.
for (let sg of aws.ec2.SecurityGroup.Query(instances = 0)) { // Retrieve all orphaned SGs. Don't know if possible.
    rescNames.push(sg.GroupName); // Add SG name to array.
} // TODO: If possible, edit this after QUERY implementation to explicitly retrieve
  // SGs without any attached instances.

// For-of loop only works if elements are Strings or Arrays.
// So, get all names of orphaned security groups, like in securitygroup_del.py
// Iterate through them here.
for (let name of rescNames){
    aws.ec2.SecurityGroup.Delete(GroupName = name); // Delete SGs with flagged GroupNames.
}
