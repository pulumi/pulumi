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
import json
from typing import Optional, Any

from . import errors
from .output import Output
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
    """
    The configuration bag's logical name that uniquely identifies it.  The default is the name of the current project.
    """

    def __init__(self, name: Optional[str] = None) -> None:
        """
        :param str name: The configuration bag's logical name that uniquely identifies it.  If not provided, the name
               of the current project is used.
        """
        if not name:
            name = get_project()
        if not isinstance(name, str):
            raise TypeError('Expected name to be a string')
        self.name = name

    def get(self, key: str) -> Optional[str]:
        """
        Returns an optional configuration value by its key, or None if it doesn't exist.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[str]
        """
        return get_config(self.full_key(key))

    def get_secret(self, key: str) -> Optional[Output[str]]:
        """
        Returns an optional configuration value by its key, marked as a secret, or None if it doesn't exist.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[str]
        """
        c = self.get(key)
        if c is None:
            return None

        return Output.secret(c)

    def get_bool(self, key: str) -> Optional[bool]:
        """
        Returns an optional configuration value, as a bool, by its key, or None if it doesn't exist.
        If the configuration value isn't a legal boolean, this function will throw an error.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[bool]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to bool.
        """
        v = self.get(key)
        if v is None:
            return None
        if v in ['true', 'True']:
            return True
        if v in ['false', 'False']:
            return False
        raise ConfigTypeError(self.full_key(key), v, 'bool')

    def get_secret_bool(self, key: str) -> Optional[Output[bool]]:
        """
        Returns an optional configuration value, as a bool, by its key, marked as a secret or None if it doesn't exist.
        If the configuration value isn't a legal boolean, this function will throw an error.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[bool]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to bool.
        """
        v = self.get_bool(key)
        if v is None:
            return None

        return Output.secret(v)

    def get_int(self, key: str) -> Optional[int]:
        """
        Returns an optional configuration value, as an int, by its key, or None if it doesn't exist.
        If the configuration value isn't a legal int, this function will throw an error.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[int]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to int.
        """
        v = self.get(key)
        if v is None:
            return None
        try:
            return int(v)
        except:
            raise ConfigTypeError(self.full_key(key), v, 'int')

    def get_secret_int(self, key: str) -> Optional[Output[int]]:
        """
        Returns an optional configuration value, as an int, by its key, marked as a secret, or None if it doesn't exist.
        If the configuration value isn't a legal int, this function will throw an error.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[int]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to int.
        """
        v = self.get_int(key)
        if v is None:
            return None

        return Output.secret(v)

    def get_float(self, key: str) -> Optional[float]:
        """
        Returns an optional configuration value, as a float, by its key, or None if it doesn't exist.
        If the configuration value isn't a legal float, this function will throw an error.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[float]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        v = self.get(key)
        if v is None:
            return None
        try:
            return float(v)
        except:
            raise ConfigTypeError(self.full_key(key), v, 'float')

    def get_secret_float(self, key: str) -> Optional[Output[float]]:
        """
        Returns an optional configuration value, as a float, by its key, marked as a secret or None if it doesn't exist.
        If the configuration value isn't a legal float, this function will throw an error.

        :param str key: The requested configuration key.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[float]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        v = self.get_float(key)
        if v is None:
            return None

        return Output.secret(v)

    def get_object(self, key: str) -> Optional[Any]:
        """
        Returns an optional configuration value, as an object, by its key, or undefined if it
        doesn't exist. This routine simply JSON parses and doesn't validate the shape of the
        contents.
        """
        v = self.get(key)
        if v is None:
            return None
        try:
            return json.loads(v)
        except:
            raise ConfigTypeError(self.full_key(key), v, "JSON object")

    def get_secret_object(self, key: str) -> Optional[Output[Any]]:
        """
        Returns an optional configuration value, as an object, by its key, marking it as a secret or
        undefined if it doesn't exist. This routine simply JSON parses and doesn't validate the
        shape of the contents.
        """
        v = self.get_object(key)
        if v is None:
            return None
        return Output.secret(v)

    def require(self, key: str) -> str:
        """
        Returns a configuration value by its given key.  If it doesn't exist, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: str
        :raises ConfigMissingError: The configuration value did not exist.
        """
        v = self.get(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_secret(self, key: str) -> Output[str]:
        """
        Returns a configuration value, marked as a secret by its given key.  If it doesn't exist, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: str
        :raises ConfigMissingError: The configuration value did not exist.
        """
        return Output.secret(self.require(key))

    def require_bool(self, key: str) -> bool:
        """
        Returns a configuration value, as a bool, by its given key.  If it doesn't exist, or the
        configuration value is not a legal bool, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: bool
        :raises ConfigMissingError: The configuration value did not exist.
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to bool.
        """
        v = self.get_bool(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_secret_bool(self, key: str) -> Output[bool]:
        """
        Returns a configuration value, as a bool, marked as a secret by its given key.  If it doesn't exist, or the
        configuration value is not a legal bool, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: bool
        :raises ConfigMissingError: The configuration value did not exist.
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to bool.
        """
        return Output.secret(self.require_bool(key))

    def require_int(self, key: str) -> int:
        """
        Returns a configuration value, as an int, by its given key.  If it doesn't exist, or the
        configuration value is not a legal int, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: int
        :raises ConfigMissingError: The configuration value did not exist.
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to int.
        """
        v = self.get_int(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_secret_int(self, key: str) -> Output[int]:
        """
        Returns a configuration value, as an int, marked as a secret by its given key.  If it doesn't exist, or the
        configuration value is not a legal int, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: int
        :raises ConfigMissingError: The configuration value did not exist.
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to int.
        """
        return Output.secret(self.require_int(key))

    def require_float(self, key: str) -> float:
        """
        Returns a configuration value, as a float, by its given key.  If it doesn't exist, or the
        configuration value is not a legal number, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: float
        :raises ConfigMissingError: The configuration value did not exist.
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        v = self.get_float(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_secret_float(self, key: str) -> Output[float]:
        """
        Returns a configuration value, as a float, marked as a secret by its given key.  If it doesn't exist, or the
        configuration value is not a legal number, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: float
        :raises ConfigMissingError: The configuration value did not exist.
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        return Output.secret(self.require_float(key))

    def require_object(self, key: str) -> Any:
        """
        Returns a configuration value as a JSON string and deserializes the JSON into a Python
        object. If it doesn't exist, or the configuration value is not a legal JSON string, an error
        is thrown.
        """
        v = self.get_object(key)
        if v is None:
            raise ConfigMissingError(self.full_key(key))
        return v

    def require_secret_object(self, key: str) -> Output[Any]:
        """
        Returns a configuration value as a JSON string and deserializes the JSON into a Python
        object, marking it as a secret. If it doesn't exist, or the configuration value is not a
        legal JSON string, an error is thrown.
        """
        return Output.secret(self.require_object(key))

    def full_key(self, key: str) -> str:
        """
        Turns a simple configuration key into a fully resolved one, by prepending the bag's name.

        :param str key: The name of the configuration key.
        :return: The name of the configuration key, prefixed with the bag's name.
        :rtype: str
        """
        return '%s:%s' % (self.name, key)


class ConfigTypeError(errors.RunError):
    """
    Indicates a configuration value is of the wrong type.
    """

    key: str
    """
    The name of the key whose value was ill-typed.
    """

    value: str
    """
    The ill-typed value.
    """

    expect_type: str
    """
    The expected type of this value.
    """

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
    """
    The name of the missing configuration key.
    """

    def __init__(self, key: str) -> None:
        self.key = key
        super(ConfigMissingError, self).__init__(
            "Missing required configuration variable '%s'\n" % key +
            "    please set a value using the command `pulumi config set %s <value>`" % key)
