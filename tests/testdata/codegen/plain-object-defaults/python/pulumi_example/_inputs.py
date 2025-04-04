# coding=utf-8
# *** WARNING: this file was generated by test. ***
# *** Do not edit by hand unless you're certain you know what you are doing! ***

import builtins
import copy
import warnings
import sys
import pulumi
import pulumi.runtime
from typing import Any, Mapping, Optional, Sequence, Union, overload
if sys.version_info >= (3, 11):
    from typing import NotRequired, TypedDict, TypeAlias
else:
    from typing_extensions import NotRequired, TypedDict, TypeAlias
from . import _utilities
from . import mod1 as _mod1
from . import mod2 as _mod2

__all__ = [
    'HelmReleaseSettings',
    'HelmReleaseSettingsDict',
    'HelmReleaseSettingsArgs',
    'HelmReleaseSettingsArgsDict',
    'KubeClientSettingsArgs',
    'KubeClientSettingsArgsDict',
    'LayeredTypeArgs',
    'LayeredTypeArgsDict',
    'TypArgs',
    'TypArgsDict',
]

MYPY = False

if not MYPY:
    class HelmReleaseSettingsDict(TypedDict):
        """
        BETA FEATURE - Options to configure the Helm Release resource.
        """
        required_arg: builtins.str
        """
        to test required args
        """
        driver: NotRequired[builtins.str]
        """
        The backend storage driver for Helm. Values are: configmap, secret, memory, sql.
        """
        plugins_path: NotRequired[builtins.str]
        """
        The path to the helm plugins directory.
        """
elif False:
    HelmReleaseSettingsDict: TypeAlias = Mapping[str, Any]

@pulumi.input_type
class HelmReleaseSettings:
    def __init__(__self__, *,
                 required_arg: builtins.str,
                 driver: Optional[builtins.str] = None,
                 plugins_path: Optional[builtins.str] = None):
        """
        BETA FEATURE - Options to configure the Helm Release resource.
        :param builtins.str required_arg: to test required args
        :param builtins.str driver: The backend storage driver for Helm. Values are: configmap, secret, memory, sql.
        :param builtins.str plugins_path: The path to the helm plugins directory.
        """
        pulumi.set(__self__, "required_arg", required_arg)
        if driver is None:
            driver = (_utilities.get_env('PULUMI_K8S_HELM_DRIVER') or 'secret')
        if driver is not None:
            pulumi.set(__self__, "driver", driver)
        if plugins_path is None:
            plugins_path = _utilities.get_env('PULUMI_K8S_HELM_PLUGINS_PATH')
        if plugins_path is not None:
            pulumi.set(__self__, "plugins_path", plugins_path)

    @property
    @pulumi.getter(name="requiredArg")
    def required_arg(self) -> builtins.str:
        """
        to test required args
        """
        return pulumi.get(self, "required_arg")

    @required_arg.setter
    def required_arg(self, value: builtins.str):
        pulumi.set(self, "required_arg", value)

    @property
    @pulumi.getter
    def driver(self) -> Optional[builtins.str]:
        """
        The backend storage driver for Helm. Values are: configmap, secret, memory, sql.
        """
        return pulumi.get(self, "driver")

    @driver.setter
    def driver(self, value: Optional[builtins.str]):
        pulumi.set(self, "driver", value)

    @property
    @pulumi.getter(name="pluginsPath")
    def plugins_path(self) -> Optional[builtins.str]:
        """
        The path to the helm plugins directory.
        """
        return pulumi.get(self, "plugins_path")

    @plugins_path.setter
    def plugins_path(self, value: Optional[builtins.str]):
        pulumi.set(self, "plugins_path", value)


if not MYPY:
    class HelmReleaseSettingsArgsDict(TypedDict):
        """
        BETA FEATURE - Options to configure the Helm Release resource.
        """
        required_arg: pulumi.Input[builtins.str]
        """
        to test required args
        """
        driver: NotRequired[pulumi.Input[builtins.str]]
        """
        The backend storage driver for Helm. Values are: configmap, secret, memory, sql.
        """
        plugins_path: NotRequired[pulumi.Input[builtins.str]]
        """
        The path to the helm plugins directory.
        """
elif False:
    HelmReleaseSettingsArgsDict: TypeAlias = Mapping[str, Any]

@pulumi.input_type
class HelmReleaseSettingsArgs:
    def __init__(__self__, *,
                 required_arg: pulumi.Input[builtins.str],
                 driver: Optional[pulumi.Input[builtins.str]] = None,
                 plugins_path: Optional[pulumi.Input[builtins.str]] = None):
        """
        BETA FEATURE - Options to configure the Helm Release resource.
        :param pulumi.Input[builtins.str] required_arg: to test required args
        :param pulumi.Input[builtins.str] driver: The backend storage driver for Helm. Values are: configmap, secret, memory, sql.
        :param pulumi.Input[builtins.str] plugins_path: The path to the helm plugins directory.
        """
        pulumi.set(__self__, "required_arg", required_arg)
        if driver is None:
            driver = (_utilities.get_env('PULUMI_K8S_HELM_DRIVER') or 'secret')
        if driver is not None:
            pulumi.set(__self__, "driver", driver)
        if plugins_path is None:
            plugins_path = _utilities.get_env('PULUMI_K8S_HELM_PLUGINS_PATH')
        if plugins_path is not None:
            pulumi.set(__self__, "plugins_path", plugins_path)

    @property
    @pulumi.getter(name="requiredArg")
    def required_arg(self) -> pulumi.Input[builtins.str]:
        """
        to test required args
        """
        return pulumi.get(self, "required_arg")

    @required_arg.setter
    def required_arg(self, value: pulumi.Input[builtins.str]):
        pulumi.set(self, "required_arg", value)

    @property
    @pulumi.getter
    def driver(self) -> Optional[pulumi.Input[builtins.str]]:
        """
        The backend storage driver for Helm. Values are: configmap, secret, memory, sql.
        """
        return pulumi.get(self, "driver")

    @driver.setter
    def driver(self, value: Optional[pulumi.Input[builtins.str]]):
        pulumi.set(self, "driver", value)

    @property
    @pulumi.getter(name="pluginsPath")
    def plugins_path(self) -> Optional[pulumi.Input[builtins.str]]:
        """
        The path to the helm plugins directory.
        """
        return pulumi.get(self, "plugins_path")

    @plugins_path.setter
    def plugins_path(self, value: Optional[pulumi.Input[builtins.str]]):
        pulumi.set(self, "plugins_path", value)


if not MYPY:
    class KubeClientSettingsArgsDict(TypedDict):
        """
        Options for tuning the Kubernetes client used by a Provider.
        """
        burst: NotRequired[pulumi.Input[builtins.int]]
        """
        Maximum burst for throttle. Default value is 10.
        """
        qps: NotRequired[pulumi.Input[builtins.float]]
        """
        Maximum queries per second (QPS) to the API server from this client. Default value is 5.
        """
        rec_test: NotRequired[pulumi.Input['KubeClientSettingsArgsDict']]
elif False:
    KubeClientSettingsArgsDict: TypeAlias = Mapping[str, Any]

@pulumi.input_type
class KubeClientSettingsArgs:
    def __init__(__self__, *,
                 burst: Optional[pulumi.Input[builtins.int]] = None,
                 qps: Optional[pulumi.Input[builtins.float]] = None,
                 rec_test: Optional[pulumi.Input['KubeClientSettingsArgs']] = None):
        """
        Options for tuning the Kubernetes client used by a Provider.
        :param pulumi.Input[builtins.int] burst: Maximum burst for throttle. Default value is 10.
        :param pulumi.Input[builtins.float] qps: Maximum queries per second (QPS) to the API server from this client. Default value is 5.
        """
        if burst is None:
            burst = _utilities.get_env_int('PULUMI_K8S_CLIENT_BURST')
        if burst is not None:
            pulumi.set(__self__, "burst", burst)
        if qps is None:
            qps = _utilities.get_env_float('PULUMI_K8S_CLIENT_QPS')
        if qps is not None:
            pulumi.set(__self__, "qps", qps)
        if rec_test is not None:
            pulumi.set(__self__, "rec_test", rec_test)

    @property
    @pulumi.getter
    def burst(self) -> Optional[pulumi.Input[builtins.int]]:
        """
        Maximum burst for throttle. Default value is 10.
        """
        return pulumi.get(self, "burst")

    @burst.setter
    def burst(self, value: Optional[pulumi.Input[builtins.int]]):
        pulumi.set(self, "burst", value)

    @property
    @pulumi.getter
    def qps(self) -> Optional[pulumi.Input[builtins.float]]:
        """
        Maximum queries per second (QPS) to the API server from this client. Default value is 5.
        """
        return pulumi.get(self, "qps")

    @qps.setter
    def qps(self, value: Optional[pulumi.Input[builtins.float]]):
        pulumi.set(self, "qps", value)

    @property
    @pulumi.getter(name="recTest")
    def rec_test(self) -> Optional[pulumi.Input['KubeClientSettingsArgs']]:
        return pulumi.get(self, "rec_test")

    @rec_test.setter
    def rec_test(self, value: Optional[pulumi.Input['KubeClientSettingsArgs']]):
        pulumi.set(self, "rec_test", value)


if not MYPY:
    class LayeredTypeArgsDict(TypedDict):
        """
        Make sure that defaults propagate through types
        """
        other: pulumi.Input['HelmReleaseSettingsArgsDict']
        thinker: pulumi.Input[builtins.str]
        """
        To ask and answer
        """
        answer: NotRequired[pulumi.Input[builtins.float]]
        """
        The answer to the question
        """
        plain_other: NotRequired['HelmReleaseSettingsArgsDict']
        """
        Test how plain types interact
        """
        question: NotRequired[pulumi.Input[builtins.str]]
        """
        The question already answered
        """
        recursive: NotRequired[pulumi.Input['LayeredTypeArgsDict']]
elif False:
    LayeredTypeArgsDict: TypeAlias = Mapping[str, Any]

@pulumi.input_type
class LayeredTypeArgs:
    def __init__(__self__, *,
                 other: pulumi.Input['HelmReleaseSettingsArgs'],
                 thinker: Optional[pulumi.Input[builtins.str]] = None,
                 answer: Optional[pulumi.Input[builtins.float]] = None,
                 plain_other: Optional['HelmReleaseSettingsArgs'] = None,
                 question: Optional[pulumi.Input[builtins.str]] = None,
                 recursive: Optional[pulumi.Input['LayeredTypeArgs']] = None):
        """
        Make sure that defaults propagate through types
        :param pulumi.Input[builtins.str] thinker: To ask and answer
        :param pulumi.Input[builtins.float] answer: The answer to the question
        :param 'HelmReleaseSettingsArgs' plain_other: Test how plain types interact
        :param pulumi.Input[builtins.str] question: The question already answered
        """
        pulumi.set(__self__, "other", other)
        if thinker is None:
            thinker = 'not a good interaction'
        pulumi.set(__self__, "thinker", thinker)
        if answer is None:
            answer = 42
        if answer is not None:
            pulumi.set(__self__, "answer", answer)
        if plain_other is not None:
            pulumi.set(__self__, "plain_other", plain_other)
        if question is None:
            question = (_utilities.get_env('PULUMI_THE_QUESTION') or '<unknown>')
        if question is not None:
            pulumi.set(__self__, "question", question)
        if recursive is not None:
            pulumi.set(__self__, "recursive", recursive)

    @property
    @pulumi.getter
    def other(self) -> pulumi.Input['HelmReleaseSettingsArgs']:
        return pulumi.get(self, "other")

    @other.setter
    def other(self, value: pulumi.Input['HelmReleaseSettingsArgs']):
        pulumi.set(self, "other", value)

    @property
    @pulumi.getter
    def thinker(self) -> pulumi.Input[builtins.str]:
        """
        To ask and answer
        """
        return pulumi.get(self, "thinker")

    @thinker.setter
    def thinker(self, value: pulumi.Input[builtins.str]):
        pulumi.set(self, "thinker", value)

    @property
    @pulumi.getter
    def answer(self) -> Optional[pulumi.Input[builtins.float]]:
        """
        The answer to the question
        """
        return pulumi.get(self, "answer")

    @answer.setter
    def answer(self, value: Optional[pulumi.Input[builtins.float]]):
        pulumi.set(self, "answer", value)

    @property
    @pulumi.getter(name="plainOther")
    def plain_other(self) -> Optional['HelmReleaseSettingsArgs']:
        """
        Test how plain types interact
        """
        return pulumi.get(self, "plain_other")

    @plain_other.setter
    def plain_other(self, value: Optional['HelmReleaseSettingsArgs']):
        pulumi.set(self, "plain_other", value)

    @property
    @pulumi.getter
    def question(self) -> Optional[pulumi.Input[builtins.str]]:
        """
        The question already answered
        """
        return pulumi.get(self, "question")

    @question.setter
    def question(self, value: Optional[pulumi.Input[builtins.str]]):
        pulumi.set(self, "question", value)

    @property
    @pulumi.getter
    def recursive(self) -> Optional[pulumi.Input['LayeredTypeArgs']]:
        return pulumi.get(self, "recursive")

    @recursive.setter
    def recursive(self, value: Optional[pulumi.Input['LayeredTypeArgs']]):
        pulumi.set(self, "recursive", value)


if not MYPY:
    class TypArgsDict(TypedDict):
        """
        A test for namespaces (mod main)
        """
        mod1: NotRequired[pulumi.Input['_mod1.TypArgsDict']]
        mod2: NotRequired[pulumi.Input['_mod2.TypArgsDict']]
        val: NotRequired[pulumi.Input[builtins.str]]
elif False:
    TypArgsDict: TypeAlias = Mapping[str, Any]

@pulumi.input_type
class TypArgs:
    def __init__(__self__, *,
                 mod1: Optional[pulumi.Input['_mod1.TypArgs']] = None,
                 mod2: Optional[pulumi.Input['_mod2.TypArgs']] = None,
                 val: Optional[pulumi.Input[builtins.str]] = None):
        """
        A test for namespaces (mod main)
        """
        if mod1 is not None:
            pulumi.set(__self__, "mod1", mod1)
        if mod2 is not None:
            pulumi.set(__self__, "mod2", mod2)
        if val is None:
            val = 'mod main'
        if val is not None:
            pulumi.set(__self__, "val", val)

    @property
    @pulumi.getter
    def mod1(self) -> Optional[pulumi.Input['_mod1.TypArgs']]:
        return pulumi.get(self, "mod1")

    @mod1.setter
    def mod1(self, value: Optional[pulumi.Input['_mod1.TypArgs']]):
        pulumi.set(self, "mod1", value)

    @property
    @pulumi.getter
    def mod2(self) -> Optional[pulumi.Input['_mod2.TypArgs']]:
        return pulumi.get(self, "mod2")

    @mod2.setter
    def mod2(self, value: Optional[pulumi.Input['_mod2.TypArgs']]):
        pulumi.set(self, "mod2", value)

    @property
    @pulumi.getter
    def val(self) -> Optional[pulumi.Input[builtins.str]]:
        return pulumi.get(self, "val")

    @val.setter
    def val(self, value: Optional[pulumi.Input[builtins.str]]):
        pulumi.set(self, "val", value)


