# Copyright 2026, Pulumi Corporation.
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

import pickle
import sys

import pytest

from pulumi.dynamic import ResourceProvider
from pulumi.dynamic.dynamic import serialize_provider


class SettingsProvider(ResourceProvider):
    def __init__(self, settings):
        self.settings = settings


class Unpicklable:
    def __reduce__(self):
        raise TypeError("cannot pickle Unpicklable")


def test_serialize_provider_restores_pickler():
    pickler = pickle.Pickler
    batch_setitems = pickle._Pickler._batch_setitems
    save_dict = pickle._Pickler.save_dict

    serialize_provider(SettingsProvider({"b": 2, "a": 1}))

    assert pickle.Pickler is pickler
    assert pickle._Pickler._batch_setitems is batch_setitems
    assert pickle._Pickler.save_dict is save_dict


def test_serialize_provider_restores_pickler_when_pickling_fails():
    pickler = pickle.Pickler
    batch_setitems = pickle._Pickler._batch_setitems
    save_dict = pickle._Pickler.save_dict

    with pytest.raises(TypeError, match="cannot pickle Unpicklable"):
        serialize_provider(SettingsProvider({"value": Unpicklable()}))

    assert pickle.Pickler is pickler
    assert pickle._Pickler._batch_setitems is batch_setitems
    assert pickle._Pickler.save_dict is save_dict


def test_serialize_provider_many_times_does_not_exhaust_recursion():
    provider = SettingsProvider({"a": {"b": {"c": {"d": {"e": 1}}}}})

    for _ in range(sys.getrecursionlimit()):
        serialize_provider(provider)


def test_serialize_provider_sorts_dicts():
    first = serialize_provider(SettingsProvider({"a": 1, "b": 2, "c": 3}))
    second = serialize_provider(SettingsProvider({"c": 3, "b": 2, "a": 1}))

    assert first == second
