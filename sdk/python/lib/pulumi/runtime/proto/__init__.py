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

"""
The Pulumi SDK runtime's Protobufs and gRPC stubs.  These are meant for internal use only.
"""

from __future__ import absolute_import

from .analyzer_pb2 import *
from .analyzer_pb2_grpc import *
from .engine_pb2 import *
from .engine_pb2_grpc import *
from .language_pb2 import *
from .language_pb2_grpc import *
from .plugin_pb2 import *
from .plugin_pb2_grpc import *
from .provider_pb2 import *
from .provider_pb2_grpc import *
from .resource_pb2 import *
from .resource_pb2_grpc import *
