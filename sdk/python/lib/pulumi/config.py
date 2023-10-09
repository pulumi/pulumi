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
from typing import Any, Callable, Optional

from . import errors, log
from .metadata import get_project
from .output import Output
from .runtime.config import get_config, is_config_secret


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
            raise TypeError("Expected name to be a string")
        self.name = name

    # pylint: disable=unused-argument
    def _get(
        self,
        key: str,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> Optional[str]:
        full_key = self.full_key(key)
        # TODO[pulumi/pulumi#7127]: Re-enabled the warning.
        # if use is not None and is_config_secret(full_key):
        #     assert instead_of is not None
        #     log.warn(
        #         f"Configuration '{full_key}' value is a secret; " +
        #         f"use `{use.__name__}` instead of `{instead_of.__name__}`")
        return get_config(full_key)

    def get(self, key: str, default: Optional[str] = None) -> Optional[str]:
        """
        Returns an optional configuration value by its key,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.

        :param str key: The requested configuration key.
        :param Optional[str] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[str]
        """
        config_candidate = self._get(key, self.get_secret, self.get)
        return config_candidate if config_candidate is not None else default

    def get_secret(
        self, key: str, default: Optional[str] = None
    ) -> Optional[Output[str]]:
        """
        Returns an optional configuration value by its key, marked as a secret,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.

        :param str key: The requested configuration key.
        :param Optional[str] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[str]
        """
        config_candidate = self._get(key)
        v = config_candidate if config_candidate is not None else default
        if v is None:
            return None
        return Output.secret(v)

    def _get_bool(
        self,
        key: str,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> Optional[bool]:
        v = self._get(key, use, instead_of)
        if v is None:
            return None
        if v in ["true", "True"]:
            return True
        if v in ["false", "False"]:
            return False
        raise ConfigTypeError(self.full_key(key), v, "bool")

    def get_bool(self, key: str, default: Optional[bool] = None) -> Optional[bool]:
        """
        Returns an optional configuration value, as a bool, by its key,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.
        If the configuration value isn't a legal boolean, this function will throw an error.

        :param str key: The requested configuration key.
        :param Optional[bool] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[bool]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to bool.
        """
        config_candidate = self._get_bool(key, self.get_secret_bool, self.get_bool)
        return config_candidate if config_candidate is not None else default

    def get_secret_bool(
        self, key: str, default: Optional[bool] = None
    ) -> Optional[Output[bool]]:
        """
        Returns an optional configuration value, as a bool, by its key, marked as a secret,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.
        If the configuration value isn't a legal boolean, this function will throw an error.

        :param str key: The requested configuration key.
        :param Optional[bool] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[bool]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to bool.
        """
        config_candidate = self._get_bool(key)
        v = config_candidate if config_candidate is not None else default
        if v is None:
            return None
        return Output.secret(v)

    def _get_int(
        self,
        key: str,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> Optional[int]:
        v = self._get(key, use, instead_of)
        if v is None:
            return None
        try:
            return int(v)
        except Exception as e:
            raise ConfigTypeError(self.full_key(key), v, "int") from e

    def get_int(self, key: str, default: Optional[int] = None) -> Optional[int]:
        """
        Returns an optional configuration value, as an int, by its key,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.
        If the configuration value isn't a legal int, this function will throw an error.

        :param str key: The requested configuration key.
        :param Optional[int] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[int]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to int.
        """
        config_candidate = self._get_int(key, self.get_secret_int, self.get_int)
        return config_candidate if config_candidate is not None else default

    def get_secret_int(
        self, key: str, default: Optional[int] = None
    ) -> Optional[Output[int]]:
        """
        Returns an optional configuration value, as an int, by its key, marked as a secret,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.
        If the configuration value isn't a legal int, this function will throw an error.

        :param str key: The requested configuration key.
        :param Optional[int] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[int]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to int.
        """
        config_candidate = self._get_int(key)
        v = config_candidate if config_candidate is not None else default
        if v is None:
            return None
        return Output.secret(v)

    def _get_float(
        self,
        key: str,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> Optional[float]:
        v = self._get(key, use, instead_of)
        if v is None:
            return None
        try:
            return float(v)
        except Exception as e:
            raise ConfigTypeError(self.full_key(key), v, "float") from e

    def get_float(self, key: str, default: Optional[float] = None) -> Optional[float]:
        """
        Returns an optional configuration value, as a float, by its key, marked as a secret,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.
        If the configuration value isn't a legal float, this function will throw an error.

        :param str key: The requested configuration key.
        :param Optional[float] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[float]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        config_candidate = self._get_float(key, self.get_secret_float, self.get_float)
        return config_candidate if config_candidate is not None else default

    def get_secret_float(
        self, key: str, default: Optional[float] = None
    ) -> Optional[Output[float]]:
        """
        Returns an optional configuration value, as a float, by its key, marked as a secret,
        a default value if that key is unset and a default is provided,
        or None if it doesn't exist.
        If the configuration value isn't a legal float, this function will throw an error.

        :param str key: The requested configuration key.
        :param Optional[float] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[float]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        config_candidate = self._get_float(key)
        v = config_candidate if config_candidate is not None else default
        if v is None:
            return None
        return Output.secret(v)

    def _get_object(
        self,
        key: str,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> Optional[Any]:
        v = self._get(key, use, instead_of)
        if v is None:
            return None
        try:
            return json.loads(v)
        except Exception as e:
            raise ConfigTypeError(self.full_key(key), v, "JSON object") from e

    def get_object(self, key: str, default: Optional[Any] = None) -> Optional[Any]:
        """
        Returns an optional configuration value, as an object, by its key,
        a default value if that key is unset and a default is provided, or undefined if it
        doesn't exist. This routine simply JSON parses and doesn't validate the shape of the
        contents.

        :param str key: The requested configuration key.
        :param Optional[Any] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[Any]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        config_candidate = self._get_object(
            key, self.get_secret_object, self.get_object
        )
        return config_candidate if config_candidate is not None else default

    def get_secret_object(
        self, key: str, default: Optional[Any] = None
    ) -> Optional[Output[Any]]:
        """
        Returns an optional configuration value, as an object, by its key, marking it as a secret,
        a default value if that key is unset and a default is provided,
        or undefined if it doesn't exist. This routine simply JSON parses and doesn't validate the
        shape of the contents.

        :param str key: The requested configuration key.
        :param Optional[Any] default: An optional fallback value to use if the given configuration key is not set.
        :return: The configuration key's value, or None if one does not exist.
        :rtype: Optional[Any]
        :raises ConfigTypeError: The configuration value existed but couldn't be coerced to float.
        """
        config_candidate = self._get_object(key)
        v = config_candidate if config_candidate is not None else default
        if v is None:
            return None
        return Output.secret(v)

    def _require(
        self,
        key: str,
        secret: bool,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> str:
        v = self._get(key, use, instead_of)
        if v is None:
            raise ConfigMissingError(self.full_key(key), secret)
        return v

    def require(self, key: str) -> str:
        """
        Returns a configuration value by its given key.  If it doesn't exist, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: str
        :raises ConfigMissingError: The configuration value did not exist.
        """
        return self._require(key, False, self.require_secret, self.require)

    def require_secret(self, key: str) -> Output[str]:
        """
        Returns a configuration value, marked as a secret by its given key.  If it doesn't exist, an error is thrown.

        :param str key: The requested configuration key.
        :return: The configuration key's value.
        :rtype: str
        :raises ConfigMissingError: The configuration value did not exist.
        """
        return Output.secret(self._require(key, True))

    def _require_bool(
        self,
        key: str,
        secret: bool,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> bool:
        v = self._get_bool(key, use, instead_of)
        if v is None:
            raise ConfigMissingError(self.full_key(key), secret)
        return v

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
        return self._require_bool(
            key, False, self.require_secret_bool, self.require_bool
        )

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
        return Output.secret(self._require_bool(key, True))

    def _require_int(
        self,
        key: str,
        secret: bool,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> int:
        v = self._get_int(key, use, instead_of)
        if v is None:
            raise ConfigMissingError(self.full_key(key), secret)
        return v

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
        return self._require_int(key, False, self.require_secret_int, self.require_int)

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
        return Output.secret(self._require_int(key, True))

    def _require_float(
        self,
        key: str,
        secret: bool,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> float:
        v = self._get_float(key, use, instead_of)
        if v is None:
            raise ConfigMissingError(self.full_key(key), secret)
        return v

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
        return self._require_float(
            key, False, self.require_secret_float, self.require_float
        )

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
        return Output.secret(self._require_float(key, True))

    def _require_object(
        self,
        key: str,
        secret: bool,
        use: Optional[Callable] = None,
        instead_of: Optional[Callable] = None,
    ) -> Any:
        v = self._get_object(key, use, instead_of)
        if v is None:
            raise ConfigMissingError(self.full_key(key), secret)
        return v

    def require_object(self, key: str) -> Any:
        """
        Returns a configuration value as a JSON string and deserializes the JSON into a Python
        object. If it doesn't exist, or the configuration value is not a legal JSON string, an error
        is thrown.
        """
        return self._require_object(
            key, False, self.require_secret_object, self.require_object
        )

    def require_secret_object(self, key: str) -> Output[Any]:
        """
        Returns a configuration value as a JSON string and deserializes the JSON into a Python
        object, marking it as a secret. If it doesn't exist, or the configuration value is not a
        legal JSON string, an error is thrown.
        """
        return Output.secret(self._require_object(key, True))

    def full_key(self, key: str) -> str:
        """
        Turns a simple configuration key into a fully resolved one, by prepending the bag's name.

        :param str key: The name of the configuration key.
        :return: The name of the configuration key, prefixed with the bag's name.
        :rtype: str
        """
        return f"{self.name}:{key}"


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
        super().__init__(
            f"Configuration '{key}' value '{value}' is not a valid '{expect_type}'"
        )


class ConfigMissingError(errors.RunError):
    """
    Indicates a configuration value is missing.
    """

    key: str
    """
    The name of the missing configuration key.
    """

    secret: bool
    """
    If this is a secret configuration key.
    """

    def __init__(self, key: str, secret: bool) -> None:
        self.key = key
        self.secret = secret
        super().__init__(
            f"Missing required configuration variable '{key}'\n"
            + f"\tplease set a value using the command `pulumi config set{' --secret ' if secret else ' '}{key} <value>`"
        )
