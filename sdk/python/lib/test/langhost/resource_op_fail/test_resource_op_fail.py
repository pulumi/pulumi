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
from os import path
from ..util import LanghostTest


class UnhandledExceptionTest(LanghostTest):
    def test_unhandled_exception(self):
        self.run_test(
            program=path.join(self.base_path(), "resource_op_fail"),
            expected_error="Program exited with non-zero exit code: 1")

    def register_resource(self, _ctx, _dry_run, _ty, _name, _resource,
                          _dependencies, _parent, _custom, _protect, _provider):
        raise Exception("oh no")
