# Copyright 2016-2017, Pulumi Corporation
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

import lumi.aws

size = "t2.micro"

def main():
    group = aws.ec2.SecurityGroup("web-secgrp",
            group_description="Enable HTTP access",
            security_group_ingress=[
                aws.ec2.SecurityGroupIngressRule("tcp", 80, 80, "0.0.0.0/0")
            ])
    server = aws.ec2.Instance("web-server-www",
            instance_type=size,
            security_groups=[ group ],
            image_id=aws.ec2.get_linux_ami(size))

if __name__ == "__main__":
    main()

