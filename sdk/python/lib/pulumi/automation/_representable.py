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


class _Representable:
    """This mix-in improves the default `__repr__` to be more readable."""

    def __repr__(self):
        inputs = self.__dict__
        fields = [f"{key}={value!r}" for key, value in inputs.items()]
        fields = ", ".join(fields)
        return f"{self.__class__.__name__}({fields})"
