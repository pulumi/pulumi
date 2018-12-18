# Copyright 2016-2018, Pulumi Corporation.
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
from pulumi import ProviderResource, CustomResource, ComponentResource, ResourceOptions

class Provider(ProviderResource):
    def __init__(self, name, opts=None):
        ProviderResource.__init__(self, "test", name, {}, opts)

class Resource(CustomResource):
    def __init__(self, name, create_children, opts=None):
        CustomResource.__init__(self, "test:index:Resource", name, {}, opts)
        if create_children is not None:
            create_children(name, self)

class Component(ComponentResource):
    def __init__(self, name, create_children, opts=None):
        ComponentResource.__init__(self, "test:index:Component", name, {}, opts)
        create_children(name, self)

def create_resources(name, create_children=None, parent=None):
    # Use all parent defaults.
    Resource(f"{name}/r0", create_children, ResourceOptions(parent=parent))

    # Override protect
    Resource(f"{name}/r1", create_children, ResourceOptions(parent=parent, protect=False))
    Resource(f"{name}/r2", create_children, ResourceOptions(parent=parent, protect=True))

    # Override provider
    prov = Provider(f"{name}-p", ResourceOptions(parent=parent))
    Resource(f"{name}/r3", create_children, ResourceOptions(parent=parent, provider=prov))

def create_components(name, create_children=None, parent=None):
    # Use all parent defaults.
    Component(f"{name}/c0", create_children, ResourceOptions(parent=parent))

    # Override protect
    Component(f"{name}/c1", create_children, ResourceOptions(parent=parent, protect=False))
    Component(f"{name}/c2", create_children, ResourceOptions(parent=parent, protect=True))

    # Override providers
    providers = {"test": Provider(f"{name}-p", ResourceOptions(parent=parent))}
    Component(f"{name}/c3", create_children, ResourceOptions(parent=parent, providers=providers))

# Create default (unparent) resources
create_resources("unparented")

# Create singly-nested resources
create_components("single-nest", lambda name, parent: create_resources(name, None, parent))

# Create doubly-nested resources
create_components("double-nest", lambda name, parent: create_components(
    name,
    lambda name, parent: create_resources(name, None, parent),
    parent))

# Create doubly-nested resources parented to other resources
create_components("double-nest-2", lambda name, parent: create_resources(
    name,
    lambda name, parent: create_resources(name, None, parent),
    parent))
