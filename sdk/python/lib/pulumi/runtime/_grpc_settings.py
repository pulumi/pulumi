# # Copyright 2025, Pulumi Corporation.
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


# _MAX_RPC_MESSAGE_SIZE raises the gRPC Max Message size from `4194304` (4mb) to `419430400` (400mb)
_MAX_RPC_MESSAGE_SIZE = 1024 * 1024 * 400
# This is the time a message can be received by the GRPC server and wait in the queue without being handled. If there is
# blocking happening in the pulumi program, and/or there are a lot of requests and other asyncio tasks to process by the
# event loop, this can take longer than the default 30 seconds. Requests that take longer end up being cancelled,
# causing the operation to fail.
_SERVER_MAX_UNREQUESTED_TIME_IN_SERVER = 30 * 60  # half an hour in seconds
_GRPC_CHANNEL_OPTIONS = [
    ("grpc.max_receive_message_length", _MAX_RPC_MESSAGE_SIZE),
    (
        "grpc.server_max_unrequested_time_in_server",
        _SERVER_MAX_UNREQUESTED_TIME_IN_SERVER,
    ),
]
