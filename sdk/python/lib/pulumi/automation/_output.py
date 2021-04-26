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

from typing import Any, MutableMapping
from ._config import _SECRET_SENTINEL


class OutputValue:
    value: Any
    secret: bool

    def __init__(self, value: Any, secret: bool):
        self.value = value
        self.secret = secret

    def __repr__(self):
        return _SECRET_SENTINEL if self.secret else repr(self.value)


OutputMap = MutableMapping[str, OutputValue]
