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

from typing import Callable
import pickle

import cloudpickle

def serialize_function(func: Callable) -> str:
    """
    serialize_function serializes a Python function into a text form that can be loaded in another
    execution context, for example as part of a function callback associated with an AWS Lambda.
    The function serialization captures any variables captured by the function body and serializes
    those values into the generated text along with the function body.  This process is recursive,
    so that functions referenced by the body of the serialized function will themselves be
    serialized as well.  This process also deeply serializes captured object values, including
    prototype chains and property descriptors, such that the semantics of the function when
    deserialized should match the original function.
    """
    return cloudpickle.dumps(func, pickle.DEFAULT_PROTOCOL)
