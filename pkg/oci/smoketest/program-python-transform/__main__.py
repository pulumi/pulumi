# Copyright 2026, Pulumi Corporation.
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

"""The Python twin of program-transform/ (Go) and program-node-transform/ (Node).

A program that registers a RESOURCE TRANSFORM — the one place a program serves an
inbound RPC instead of only dialing out. register_resource_transform stands up a
callback gRPC server in this process and hands the engine its address, so the engine
dials back here for every resource registration.

Whether that address is reachable is the point of the test: it is built from
PULUMI_CALLBACKS_ADVERTISE_HOST when the program runs in its own network namespace,
and defaults to loopback when it shares one with the engine.

The transform sets `prefix`, so the resulting pet name is self-evidently transformed
or not — a name starting with "transformed-" proves the engine reached back in.
"""

from typing import Optional

import pulumi
import pulumi_random as random

# Injected by the transform and visible in the stack output, so "the transform ran" and
# "the transform was skipped" cannot be confused.
TRANSFORMED_PREFIX = "transformed"


def add_prefix(
    args: pulumi.ResourceTransformArgs,
) -> Optional[pulumi.ResourceTransformResult]:
    if args.type_ != "random:index/randomPet:RandomPet":
        return None
    return pulumi.ResourceTransformResult(
        props={**args.props, "prefix": TRANSFORMED_PREFIX},
        opts=args.opts,
    )


pulumi.runtime.register_resource_transform(add_prefix)

pulumi.log.info(
    "oci transform program (python) registered a resource transform (engine must dial back)"
)

pet = random.RandomPet("pet")

pulumi.export("petName", pet.id)
