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

"""A minimal Python Pulumi program that registers a *dynamic* resource, to prove
dynamic-provider execution is language-agnostic in the OCI pod model.

The dynamic provider's CRUD code is serialized from this program (via dill) and
runs in a separate provider process; in the pod model that process is a container
started FROM THIS PROGRAM'S IMAGE (the SDK's dynamic-provider entrypoint,
``python -m pulumi.dynamic``, is native to it). The welding is proven with a
marker baked into the program image: ``create`` reads ``/program-marker`` and
returns it as the resource's output. From any other image the file would be
absent and ``create`` would throw, so a single assertion (stack output == the
baked marker) proves both that the resource was created and that its provider ran
welded to the program image.
"""

import pulumi
from pulumi.dynamic import CreateResult, Resource, ResourceProvider


class MarkerProvider(ResourceProvider):
    def create(self, props):
        # Read inside the method so it travels in the serialized (dill) provider and
        # runs in the provider process — a container from this program's image.
        # /program-marker exists only in that image.
        with open("/program-marker") as f:
            marker = f.read().strip()
        return CreateResult(id_="oci-dynamic-py-1", outs={"marker": marker})


class MarkerResource(Resource):
    marker: pulumi.Output[str]

    def __init__(self, name, opts=None):
        super().__init__(MarkerProvider(), name, {"marker": None}, opts)


res = MarkerResource("oci-smoke-dynamic-py")
pulumi.export("marker", res.marker)
