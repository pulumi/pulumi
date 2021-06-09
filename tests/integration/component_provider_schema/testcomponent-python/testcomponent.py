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

from typing import Optional
import sys
import os

import pulumi.provider as provider

class Provider(provider.Provider):
    VERSION = "0.0.1"

    def __init__(self, schema: Optional[str] = None):
        super().__init__(Provider.VERSION, schema)

if __name__ == '__main__':
    schema = '{"hello": "world"}' if os.getenv("INCLUDE_SCHEMA") is not None else None
    provider.main(Provider(schema), sys.argv[1:])
