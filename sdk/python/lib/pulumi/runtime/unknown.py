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

from __future__ import absolute_import

class Unknown(str):
    """
    Unknown is a class representing values that are not known during
    previews. This class is designed to lie its way through the entire
    Pulumi runtime.

    We inherit from `unicode` to lie to resources that we are a string.
    """
    def __getattribute__(self, attr):
        """
        Called whenever a property is read from an Unknown object. A property
        access like `foo.bar` would call `Unknown.__getattribute__(foo, 'bar')`.

        Unknown claims that all properties are Unknown, which should get us through
        most previews.
        """
        return Unknown()

    def __bool__(self):
        """
        Called whenever this value is coerced to a boolean, via "bool" or the "not"
        operator. This value is not zero, so we return True.
        """
        return True

    def __nonzero__(self):
        """
        Python 2/3 compatibility shim for __bool__. Also returns True if this
        value is not zero, which it is.
        """
        return True

    def __str__(self):
        """
        Called whenever this value is coerced to a string, via "str".
        """
        return "<computed>"

    def __len__(self):
        """
        Called whenever somebody calls `len` on us, allowing us to masquerade as a list.
        """
        return 0

    def __getitem__(self, name):
        """
        Called whenever somebody uses the index operator on us, allowing us to masquerade as
        lists and dicts.
        """
        return Unknown()

    def __iter__(self):
        """
        Called whenever somebody calls `iter` on us or uses us in a `for..in` statement.
        """
        return iter([])

    def __dir__(self):
        """
        Called whenever somebody calls `dir` on us. We're lying about all of our
        properties so we say that we don't have any.
        """
        return []


