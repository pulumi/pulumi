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
The primary Pulumi Python SDK package.
"""
from __future__ import absolute_import
import six

# Make subpackages available.
__all__ = ['runtime']

# Make all module members inside of this package available as package members.
if six.PY3:
    __all__.append('next')

    # While we are converting the Pulumi SDK to Python 3, we're incrementally replacing
    # parts of the existing SDK with the "next" subpackage.
    from .next.asset import *
    from .next.config import *
    from .next.errors import *
    from .next.metadata import *
    from .next.resource import *
    from .next.log import *
else:
    from .asset import *
    from .config import *
    from .errors import *
    from .metadata import *
    from .resource import *
    from .log import *

