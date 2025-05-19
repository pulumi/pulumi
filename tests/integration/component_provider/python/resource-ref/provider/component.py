# Copyright 2025, Pulumi Corporation.
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


from typing import TypedDict

import pulumi
import pulumi_command.local as command_local


class Args(TypedDict):
    name: str
    command_in: command_local.Command
    loglevel_in: command_local.Logging


class EchoCommand(pulumi.ComponentResource):
    """
    EchoCommand has an output that is a resource. Schema inference should detect
    this and properly handle the resource output.

    This component also takes a command as an input. We should be able to use
    that resource inside the component.
    """

    command_out: pulumi.Output[command_local.Command]
    command_in_stdout: pulumi.Output[str]
    loglevel_out: pulumi.Output[command_local.Logging]

    def __init__(self, name: str, args: Args, opts: pulumi.ResourceOptions):
        super().__init__("provider:index:EchoCommand", name, {}, opts)
        self.command_out = command_local.Command(
            "echo",
            create="echo Hello, ${USER_NAME}",
            environment={
                "USER_NAME": args["name"],
            },
        )
        self.command_in_stdout = args["command_in"].stdout
        loglevel_out = args["loglevel_in"]
        # Assert that the provider correctly deserialized into the Python enum type.
        if not isinstance(loglevel_out, command_local.Logging):
            raise TypeError(
                f"loglevel_in must be a command_local.Logging, got {type(loglevel_out)}"
            )
        self.loglevel_out = loglevel_out
        self.register_outputs(
            {
                "command_out": self.command_out,
                "command_in_stdout": self.command_in_stdout,
                "loglevel_out": self.loglevel_out,
            }
        )
