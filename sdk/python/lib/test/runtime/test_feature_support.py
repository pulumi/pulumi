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

import grpc
import pytest

from pulumi.runtime import settings
from pulumi.runtime.proto import resource_pb2
from pulumi.runtime.settings import Settings, _load_monitor_feature_support


class _UnimplementedError(grpc.RpcError):
    def code(self):
        return grpc.StatusCode.UNIMPLEMENTED


class DeploymentInfoMonitor:
    """A monitor that advertises its features via GetDeploymentInfo."""

    def __init__(self, features):
        self.features = features
        self.supports_feature_calls = 0

    def GetDeploymentInfo(self, _req):
        return resource_pb2.DeploymentInfo(supportedFeatures=self.features)

    def SupportsFeature(self, req):
        self.supports_feature_calls += 1
        return resource_pb2.SupportsFeatureResponse(hasSupport=True)


class LegacyMonitor:
    """A monitor that predates GetDeploymentInfo and only answers SupportsFeature."""

    def __init__(self, features):
        self.features = features
        self.probed = []

    def GetDeploymentInfo(self, _req):
        raise _UnimplementedError()

    def SupportsFeature(self, req):
        self.probed.append(req.id)
        return resource_pb2.SupportsFeatureResponse(hasSupport=req.id in self.features)


def configure_monitor(monitor):
    s = Settings(project="project", stack="stack")
    s.monitor = monitor
    s.feature_support = {}
    settings.configure(s)


@pytest.mark.asyncio
async def test_loads_features_from_get_deployment_info():
    monitor = DeploymentInfoMonitor(
        [
            resource_pb2.RESOURCE_MONITOR_FEATURE_SECRETS,
            resource_pb2.RESOURCE_MONITOR_FEATURE_RESOURCE_REFERENCES,
            resource_pb2.RESOURCE_MONITOR_FEATURE_ALIAS_SPECS,
            resource_pb2.RESOURCE_MONITOR_FEATURE_TRANSFORMS,
            resource_pb2.RESOURCE_MONITOR_FEATURE_RESOURCE_HOOKS,
            # Features with no legacy string ID never show up in feature_support.
            resource_pb2.RESOURCE_MONITOR_FEATURE_BYTE_STRING,
        ]
    )
    configure_monitor(monitor)

    await _load_monitor_feature_support()

    assert settings.SETTINGS.feature_support.fget() == {
        "secrets": True,
        "resourceReferences": True,
        "outputValues": False,
        "deletedWith": False,
        "replaceWith": False,
        "aliasSpecs": True,
        "transforms": True,
        "invokeTransforms": False,
        "parameterization": False,
        "resourceHooks": True,
        "errorHooks": False,
    }
    assert monitor.supports_feature_calls == 0


@pytest.mark.asyncio
async def test_falls_back_to_supports_feature_when_unimplemented():
    monitor = LegacyMonitor({"secrets", "outputValues", "invokeTransforms"})
    configure_monitor(monitor)

    await _load_monitor_feature_support()

    assert settings.SETTINGS.feature_support.fget() == {
        "secrets": True,
        "resourceReferences": False,
        "outputValues": True,
        "deletedWith": False,
        "replaceWith": False,
        "aliasSpecs": False,
        "transforms": False,
        "invokeTransforms": True,
        "parameterization": False,
        "resourceHooks": False,
        "errorHooks": False,
    }
    assert sorted(monitor.probed) == [
        "aliasSpecs",
        "deletedWith",
        "errorHooks",
        "invokeTransforms",
        "outputValues",
        "parameterization",
        "replaceWith",
        "resourceHooks",
        "resourceReferences",
        "secrets",
        "transforms",
    ]
