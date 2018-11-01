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
The primary Pulumi Python SDK package, targeting Python 3.6 and above.

The pulumi.next package is a point-in-time subpackage to facilitate the
upgrade of the Pulumi Python SDK to Python 3.6. It provides Python 3-only
implementations of different parts of the SDK that are being used to replace
the Python 2 implementations in a piecemeal fashion.
"""

# Make subpackages available.
__all__ = ['runtime']

# Make all module members inside of this package available as package members.
from .asset import *
from .config import *
from .errors import *
from .metadata import *
from .resource import *
from .output import *
from .log import *
