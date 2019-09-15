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
import asyncio
from pulumi import CustomResource, Output, ResourceOptions
from pulumi.runtime import invoke

class MyResource(CustomResource):
    value: Output[str]

    def __init__(self, name, value, opts):
        CustomResource.__init__(self, "test:index:MyResource", name, props={
            "value": value,
        }, opts=opts)

# We run this invoke first because of the way in which it interacts with the RPC manager. Prior to #3170, the RPC
# manager would decide that all outstanding RPCs had finished on any non-zero -> zero transition in the number of
# outstanding RPCs. Because an invoke is considered an RPC, running any synchronous invokes before any other RPC
# would confuse this logic, which would cause us to drop exceptions that occurred during subsequent RPCs and
# incorrectly consider failed programs to have succeeded.
invoke("test:index:MyFunction", props={})

resA = MyResource("resourceA", "foo", None)

fut = asyncio.Future()
fut.set_exception(Exception("oh no"))
resB = MyResource("resourceB", fut, ResourceOptions(depends_on=[resA]))

resC = MyResource("resourceC", "baz", ResourceOptions(depends_on=[resB]))
