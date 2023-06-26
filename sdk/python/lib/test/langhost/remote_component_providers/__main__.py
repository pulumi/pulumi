# Copyright 2016-2023, Pulumi Corporation.
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

from pulumi import ComponentResource, ProviderResource, ResourceOptions


class Provider(ProviderResource):
    def __init__(self, name, opts=None):
        super().__init__("test", name, opts=opts)


class FooProvider(ProviderResource):
    def __init__(self, name, opts=None):
        super().__init__("foo", name, opts=opts)


class RemoteComponent(ComponentResource):
    def __init__(self, name, opts=None):
        super().__init__("test:index:Component", name, opts=opts, remote=True)


myprovider = Provider("myprovider")

RemoteComponent("singular", ResourceOptions(provider=myprovider))
RemoteComponent("map", ResourceOptions(providers={"test": myprovider}))
RemoteComponent("array", ResourceOptions(providers=[myprovider]))

fooprovider = FooProvider("fooprovider")

RemoteComponent("foo-singular", ResourceOptions(provider=fooprovider))
RemoteComponent("foo-map", ResourceOptions(providers={"foo": fooprovider}))
RemoteComponent("foo-array", ResourceOptions(providers=[fooprovider]))
