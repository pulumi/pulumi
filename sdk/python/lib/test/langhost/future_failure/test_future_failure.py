# Copyright 2016-2021, Pulumi Corporation.
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
from os import path
from ..util import LanghostTest


class TestFutureFailure(LanghostTest):
    def test_future_failure(self):
        self.run_test(
            program=path.join(self.base_path(), "future_failure"),
            expected_resource_count=1,
            expected_error="Program exited with non-zero exit code: 1")

    def invoke(self, _ctx, token, args, provider, _version):
        self.assertEqual("test:index:MyFunction", token)
        return [], {}

    def register_resource(self, _ctx, _dry_run, ty, name, _resource, _dependencies, _parent, _custom, protect,
                          _provider, _property_deps, _delete_before_replace, _ignore_changes, _version, _import,
                          _replace_on_changes):
        self.assertEqual("test:index:MyResource", ty)
        return {
            "urn": self.make_urn(ty, name),
            "id": name,
            "object": _resource,
        }
