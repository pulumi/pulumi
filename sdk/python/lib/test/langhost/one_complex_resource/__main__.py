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
from pulumi import CustomResource


class MyResource(CustomResource):
    def __init__(self, name):
        CustomResource.__init__(self, "test:index:MyResource", name, props={
            "falseprop": False,
            "trueprop": True,
            "intprop": 42,
            "listprop": [1, 2, "string", False],
            "mapprop": {
                "foo": ["bar", "baz"]
            }
        })

    def set_outputs(self, outputs):
        self.outprop = outputs["outprop"]
        self.outintprop = outputs["outintprop"]


res = MyResource("testres")
assert res.outprop == "output properties ftw"
assert res.outintprop == 99
