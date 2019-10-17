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
from pulumi import CustomResource, ProviderResource, ResourceOptions

class Provider(ProviderResource):
    def __init__(self, name, opts=None):
        ProviderResource.__init__(self, "test", name, {}, opts)


class Resource(CustomResource):
    def __init__(self, name, opts=None):
        CustomResource.__init__(self, "test:index:Resource", name, {}, opts)

# Create a Provider that we'll use to create other resources.
prov = Provider("testprov")

# Use this Provider to create a resource.
res = Resource("testres", ResourceOptions(provider=prov))

assert prov == res.get_provider("test:index:Resource")
