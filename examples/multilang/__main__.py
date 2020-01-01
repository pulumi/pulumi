# Copyright 2016-2018, Pulumi Corporation.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
import pulumi
from pulumi_aws import ec2

# pulumi.runtime.register_proxy_constructor("aws:ec2/securityGroup:SecurityGroup", ec2.SecurityGroup);

from mycomponent.python import MyComponent

res = MyComponent("n", input1=42)

pulumi.export("id2", res.myid)
pulumi.export("output1", res.output1)
pulumi.export("innerComponent", res.innerComponent.data)
pulumi.export("nodeSecurityGroupId", res.nodeSecurityGroup)
