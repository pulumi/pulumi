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

"""Generated component code references these helpers as `pulumi.runtime.<name>` (see
pkg/codegen/python). A user-authored subclass of a generated component -- which conformance
does not exercise, since it only tests generated-derived components -- hits the
get_type_token path, so a missing export breaks subclassing at runtime rather than at codegen
time. Assert every name the codegen emits under `pulumi.runtime` actually resolves there."""

import pulumi.runtime


def test_runtime_exports_base_construction_helpers():
    # The exact symbols emitted by the component-inheritance codegen.
    for name in ("construct_base_resource", "BaseConstructInfo", "get_type_token"):
        assert hasattr(pulumi.runtime, name), (
            f"generated component code references pulumi.runtime.{name}, but it is not exported"
        )
        assert name in pulumi.runtime.__all__, (
            f"pulumi.runtime.{name} missing from __all__"
        )
