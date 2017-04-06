# Copyright 2017 Pulumi, Inc. All rights reserved.

import json
import yaml

coconut_proj_base = "Coconut"  # the base filename for Coconut project files.
coconut_pack_base = "Cocopack" # the base filename for Coconut package files.
coconut_default_ext = ".json"  # the default extension used for Coconut markup files.

# A mapping from format extension to a function that unmarshals a blob.
unmarshalers = {
    ".json": lambda s: json.loads(s),
    ".yaml": lambda s: yaml.load(s),
}

class Package:
    """A fully compiled Coconut package definition."""
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

