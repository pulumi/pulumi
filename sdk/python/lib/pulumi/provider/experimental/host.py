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

import sys
from pathlib import Path
from typing import Optional

from ...provider import main
from .metadata import Metadata
from .provider import ComponentProvider

is_hosting = False


def component_provider_host(metadata: Optional[Metadata] = None):
    """
    component_provider_host starts the provider host for the plugin at path
    sys.argv[0]. This will discover all `pulumi.ComponentResource` sublcasses in
    the Python source code and infer their schema from their type annotations.

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
    path = Path(sys.argv[0])
    args = sys.argv[1:]

    if metadata is None:
        metadata = Metadata(path.absolute().name, "0.0.1")

    main(ComponentProvider(metadata, path), args)
