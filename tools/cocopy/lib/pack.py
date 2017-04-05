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
            dependencies=None, modules=None, aliases=None):
        self.name = name                                              # a required fully qualified name.
        if description is not None: self.description = description    # an optional informational description.
        if author is not None: self.author = author                   # an optional author email address.
        if website is not None: self.website = website                # an optional website for additional information.
        if license is not None: self.license = license                # an optional license governing usage.
        if dependencies is not None: self.dependencies = dependencies # all of the package's dependencies.
        if modules is not None: self.modules = modules                # a collection of top-level modules.
        if aliases is not None: self.aliases = aliases                # an optional map of aliased module names.

