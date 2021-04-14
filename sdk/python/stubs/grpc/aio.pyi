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

from typing import Optional, Sequence, Any
import grpc


class Server:
    def add_insecure_port(self, address: str) -> int: ...
    async def start(self) -> None: ...
    async def stop(self, grace: Optional[float]) -> None: ...
    async def wait_for_termination(self, timeout: Optional[float]=...) -> bool: ...


def server(options: Any) -> Server: ...
