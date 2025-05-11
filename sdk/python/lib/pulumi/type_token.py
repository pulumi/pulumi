# Copyright 2025, Pulumi Corporation.
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

from typing import TypeVar, Callable, Optional

T = TypeVar("T", bound=type)

_PULUMI_TYPE = "pulumi_type"


def type_token(type_token: str) -> Callable[[T], T]:
    """
    Decorate a class representing a Pulumi type like a resource, but also enums,
    with its type token.

    Example:
        @pulumi.type_token("aws:s3/bucket:Bucket")
        class Bucket(Resource): ...
    """

    def decorator(klass: T) -> T:
        setattr(klass, _PULUMI_TYPE, type_token)
        return klass

    return decorator


def get_type_token(klass: type) -> Optional[str]:
    """Retrieve the type token from a Pulumi type."""
    return getattr(klass, _PULUMI_TYPE, None)
