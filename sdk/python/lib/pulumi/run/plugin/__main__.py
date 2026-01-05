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

import importlib
import runpy
import sys

from pulumi.provider.experimental.host import (
    component_provider_host,
    components_from_module,
)


def main():
    """
    Run the Python package based plugin specified by sys.argv[1].

    We expect the package to have a module matching the package name (with `-`
    replaced by `_`). If the module exports any components, we run
    `component_provider_host` with those components. If the package does not
    export any components, we attempt to run the module's `__main__.py` file.
    """
    if len(sys.argv) < 2:
        raise Exception("Missing package name argument")

    # Get the distribution name (aka package name) from the args and remove it.
    distribution_name = sys.argv.pop(1)
    module_name = distribution_name.replace("-", "_")
    # We could provide an override here in the future, where a user can specify
    # a list of modules or components in pyproject.toml, for example
    #
    # [tool.pulumi]
    # components = ["some.module.here", "another.module:MyComponent"]
    #
    # For now we always host all the components exported from the module with the same name
    # as the distribution.
    mod = importlib.import_module(module_name)
    components = components_from_module(mod)
    if len(components) > 0:
        component_provider_host(name=distribution_name, components=components)
    else:
        # The module has no components, assume that the module is runnable.
        try:
            runpy.run_module(module_name, run_name="__main__")
        except ImportError as e:
            print(
                f"{module_name} can not be executed: {e.msg}\n\n"
                + "Please ensure that your module includes a `__main__.py` file that can be run with `python -m <module>`",
                file=sys.stderr,
                flush=True,
            )
            sys.exit(1)


if __name__ == "__main__":
    main()
