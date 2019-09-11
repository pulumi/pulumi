import abc
import datetime
import enum
import typing

import jsii
import jsii.compat
import publication

from jsii.python import classproperty
__jsii_assembly__ = jsii.JSIIAssembly.load("experiments", "1.0.0", __name__, "experiments@1.0.0.jsii.tgz")
class HelloJsii(metaclass=jsii.JSIIMeta, jsii_type="experiments.HelloJsii"):
    def __init__(self) -> None:
        jsii.create(HelloJsii, self, [])

    @jsii.member(jsii_name="baz")
    def baz(self, input: typing.Union[jsii.Number, "IOutput"]) -> typing.Union[str, "IOutput"]:
        """
        Arguments:
            input: -
        """
        return jsii.invoke(self, "baz", [input])

    @property
    @jsii.member(jsii_name="strings")
    def strings(self) -> typing.List[str]:
        return jsii.get(self, "strings")

    @strings.setter
    def strings(self, value: typing.List[str]):
        return jsii.set(self, "strings", value)


@jsii.interface(jsii_type="experiments.IOutput")
class IOutput(jsii.compat.Protocol):
    @staticmethod
    def __jsii_proxy_class__():
        return _IOutputProxy

    @jsii.member(jsii_name="val")
    def val(self) -> typing.Any:
        ...


class _IOutputProxy():
    __jsii_type__ = "experiments.IOutput"
    @jsii.member(jsii_name="val")
    def val(self) -> typing.Any:
        return jsii.invoke(self, "val", [])


__all__ = ["HelloJsii", "IOutput", "__jsii_assembly__"]

publication.publish()
