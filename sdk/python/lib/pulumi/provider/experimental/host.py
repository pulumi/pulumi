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

from inspect import isclass
import sys
from types import ModuleType
from typing import Optional

from ...resource import ComponentResource
from ...provider import main
from .component import ComponentProvider

is_hosting = False


def component_provider_host(
    components: list[type[ComponentResource]],
    name: str,
    namespace: Optional[str] = None,
):
    """
    component_provider_host starts the provider and hosts the passed in components.
    The provider's schema is inferred from the type annotations of the components.
    See `analyzer.py` for more details.

    :param metadata: The metadata for the provider. If not provided, the name
    defaults to the plugin's directory name, and version defaults to "0.0.1".
    """
    global is_hosting  # noqa
    if is_hosting:
        # Bail out if we're already hosting. This prevents recursion when the
        # analyzer loads this file. It's usually good style to not run code at
        # import time, and use `if __name__ == "__main__"`, but let's make sure
        # we guard against this.
        return
    is_hosting = True

    # When the languge runtime runs the plugin, the first argument is the path
    # to the plugin's installation directory. This is followed by the engine
    # address and other optional arguments flags, like `--logtostderr`.
    args = sys.argv[1:]
    # Default the version to "0.0.0" for now, otherwise SDK codegen gets
    # confused without a version.
    version = "0.0.0"
    main(ComponentProvider(components, name, namespace, version), args)


def components_from_module(mod: ModuleType) -> list[type[ComponentResource]]:
    components: list[type[ComponentResource]] = []
    for _, v in mod.__dict__.items():
        if isclass(v) and issubclass(v, ComponentResource):
            components.append(v)
    return components
