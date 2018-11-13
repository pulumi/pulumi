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
The config module contains all configuration management functionality.
"""
from typing import Optional

from . import errors
from .runtime.config import get_config
from .metadata import get_project


class Config:
    """
    Config is a bag of related configuration state.  Each bag contains any number of configuration variables, indexed by
    simple keys, and each has a name that uniquely identifies it; two bags with different names do not share values for
    variables that otherwise share the same key.  For example, a bag whose name is `pulumi:foo`, with keys `a`, `b`,
    and `c`, is entirely separate from a bag whose name is `pulumi:bar` with the same simple key names.  Each key has a
    fully qualified names, such as `pulumi:foo:a`, ..., and `pulumi:bar:a`, respectively.
    """

    name: str

    def __init__(self, name: str) -> None:
        if not name:
            name = get_project()
        if not isinstance(name, str):
            raise TypeError('Expected name to be a string')
        self.name = name
        """The configuration bag's logical name that uniquely identifies it.  The default is the name of the current
        project."""

    def get(self, key: str) -> Optional[str]:
        """
        Returns an optional configuration value by its key, or None if it doesn't exist.
        """
        return get_config(self.full_key(key))

    def get_bool(self, key: str) -> Optional[bool]:
        """
        Returns an optional configuration value, as a bool, by its key, or None if it doesn't exist.
        If the configuration value isn't a legal boolean, this function will throw an error.
        """
        v = self.get(key)
        if v is None:
            return None
        if v in ['true', 'True']:
            return True
        if v in ['false', 'False']:
            return False
        raise ConfigTypeError(self.full_key(key), v, 'bool')

    def get_int(self, key: str) -> Optional[int]:
        """
        Returns an optional configuration value, as an int, by its key, or None if it doesn't exist.
        If the configuration value isn't a legal int, this function will throw an error.
        """
        v = self.get(key)
        if v is None:
            return None
        try:
            return int(v)
        except:
            raise ConfigTypeError(self.full_key(key), v, 'int')

    def get_float(self, key: str) -> Optional[float]:
        """
        Returns an optional configuration value, as a float, by its key, or None if it doesn't exist.
        If the configuration value isn't a legal float, this function will throw an error.
        """
        v = self.get(key)
        if v is None:
            return None
        try:
            return float(v)
        except:
            raise ConfigTypeError(self.full_key(key), v, 'float')

    def require(self, key: str) -> str:
        """
        Returns a configuration value by its given key.  If it doesn't exist, an error is thrown.
        """
        v = self.get(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_bool(self, key: str) -> bool:
        """
        Returns a configuration value, as a bool, by its given key.  If it doesn't exist, or the
        configuration value is not a legal bool, an error is thrown.
        """
        v = self.get_bool(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_int(self, key: str) -> int:
        """
        Returns a configuration value, as an int, by its given key.  If it doesn't exist, or the
        configuration value is not a legal int, an error is thrown.
        """
        v = self.get_int(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_float(self, key: str) -> float:
        """
        Returns a configuration value, as a float, by its given key.  If it doesn't exist, or the
        configuration value is not a legal number, an error is thrown.
        """
        v = self.get_float(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def full_key(self, key: str) -> str:
        """
        Turns a simple configuration key into a fully resolved one, by prepending the bag's name.
        """
        return '%s:%s' % (self.name, key)


class ConfigTypeError(errors.RunError):
    """
    Indicates a configuration value is of the wrong type.
    """

    key: str
    value: str
    expect_type: str

    def __init__(self, key: str, value: str, expect_type: str) -> None:
        self.key = key
        self.value = value
        self.expect_type = expect_type
        super(ConfigTypeError, self).__init__(
            "Configuration '%s' value '%s' is not a valid '%s'" % (key, value, expect_type))


class ConfigMissingError(errors.RunError):
    """
    Indicates a configuration value is missing.
    """

    key: str

    def __init__(self, key: str) -> None:
        self.key = key
        super(ConfigMissingError, self).__init__(
            "Missing required configuration variable '%s'\n" % key +
            "    please set a value using the command `pulumi config set %s <value>`" % key)
