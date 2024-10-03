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

from typing import List, Optional, TypedDict


class RunError(Exception):
    """
    Can be used for terminating a program abruptly, but resulting in a clean exit rather than the usual
    verbose unhandled error logic which emits the source program text and complete stack trace.
    """


class InputPropertyError(Exception):
    def __init__(self, property_path: str, reason: str):
        """
        Can be used to indicate that the client has made a request with a bad input property.
        """
        self.property_path = property_path
        self.reason = reason


class InputPropertyErrorDetails(TypedDict):
    """
    Represents an error in a property value.
    """

    property_path: str
    reason: str


class InputPropertiesError(Exception):
    def __init__(
        self, message: str, errors: Optional[List[InputPropertyErrorDetails]] = None
    ):
        """
        Can be used to indicate that the client has made a request with bad input properties.
        """
        super().__init__(message)
        self.message = message
        self.errors = errors
