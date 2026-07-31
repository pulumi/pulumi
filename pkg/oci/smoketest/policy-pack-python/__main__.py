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

"""A minimal *Python* policy pack, the Python twin of policy-pack-node. The engine
side treats every pack identically (an image run with PULUMI_OCI_ROLE=policy-pack),
so what this fixture discriminates is the PYTHON policy SDK's serve site
(pulumi_policy/policy.py): in address mode the pack is reachable only if that serve
site honors PULUMI_PLUGIN_LISTEN_ADDRESS itself — no shim exists on the policy path.

The same two proofs as the Node pack:

  1. The pack's toolchain (python + the pulumi-policy closure) lives in THIS image;
     the engine image (alpine) has no Python at all. The pack runs only because its
     toolchain is baked into its own container.
  2. The violation message carries a marker read from /policy-marker, baked into
     this image alone, read inside the validation logic — so the marker appearing
     in a violation proves the policy evaluation ran from this image.

The flagged types cover the companion programs the smoke tests pair this pack with:
the dynamic resource either language's companion registers, and the host-mode
smoke's RandomPet. Enforcement is advisory, so `up` succeeds and prints the
violation.
"""

from pulumi_policy import (
    EnforcementLevel,
    PolicyPack,
    ReportViolation,
    ResourceValidationArgs,
    ResourceValidationPolicy,
)

_FLAGGED_TYPES = {
    "pulumi-nodejs:dynamic:Resource",
    "pulumi-python:dynamic:Resource",
    "random:index/randomPet:RandomPet",
}


def _flag_with_marker(args: ResourceValidationArgs, report_violation: ReportViolation):
    if args.resource_type in _FLAGGED_TYPES:
        # Read inside the policy logic so the read proves the *evaluation* ran in
        # this image. /policy-marker exists only here.
        with open("/policy-marker", encoding="utf-8") as f:
            marker = f.read().strip()
        report_violation(f"oci python policy ran from its image: marker={marker}")


PolicyPack(
    name="oci-policy-smoke-python",
    enforcement_level=EnforcementLevel.ADVISORY,
    policies=[
        ResourceValidationPolicy(
            name="oci-policy-smoke-python-flag",
            description="Flags the smoke tests' resources to prove the Python analyzer ran from its image.",
            validate=_flag_with_marker,
        ),
    ],
)
