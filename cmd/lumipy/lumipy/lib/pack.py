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

import json
import yaml

lumi_proj_base = "Lumi"     # the base filename for Lumi project files.
lumi_pack_base = "Lumipack" # the base filename for Lumi package files.
lumi_default_ext = ".json"  # the default extension used for Lumi markup files.

# A mapping from format extension to a function that unmarshals a blob.
unmarshalers = {
    ".json": lambda s: json.loads(s),
    ".yaml": lambda s: yaml.load(s),
}

class Package:
    """A fully compiled Lumi package definition."""
    def __init__(self, name, description=None, author=None, website=None, license=None,
            dependencies=dict(), modules=dict(), aliases=dict()):
        self.name = name                 # a required fully qualified name.
        self.description = description   # an optional informational description.
        self.author = author             # an optional author email address.
        self.website = website           # an optional website for additional information.
        self.license = license           # an optional license governing usage.
        self.dependencies = dependencies # all of the package's dependencies.
        self.modules = modules           # a collection of top-level modules.
        self.aliases = aliases           # an optional map of aliased module names.

