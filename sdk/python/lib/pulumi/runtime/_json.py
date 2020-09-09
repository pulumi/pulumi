# Copyright 2016-2020, Pulumi Corporation.
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

from collections import abc
import json
from typing import Any

from .. import _types


def _to_json_serializable_obj(obj: Any) -> Any:
    """
    Convert `obj` into an object suitable for JSON serialization, converting input types into dicts.
    """
    # Exclude some built-in types that are instances of Sequence that we don't want to treat as sequences here.
    # From: https://github.com/python/cpython/blob/master/Lib/_collections_abc.py
    if isinstance(obj, abc.Sequence) and not isinstance(obj, (tuple, str, range, memoryview, bytes, bytearray)):
        return [_to_json_serializable_obj(v) for v in obj]

    if _types.is_input_type(type(obj)):
        obj = _types.input_type_to_dict(obj)

    if isinstance(obj, abc.Mapping):
        return {k: _to_json_serializable_obj(v) for k, v in obj.items()}

    return obj


def to_json(obj: Any) -> str:
    """
    Serialize `obj` to a JSON formatted `str`.
    """
    serializable_obj = _to_json_serializable_obj(obj)
    return json.dumps(serializable_obj)
