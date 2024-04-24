import sys
import types

# Similar code is generated for the `config` module of
# generated provider Python SDKs.


class _ExportableModule(types.ModuleType):
    @property
    def foo(self) -> str:
        return "foo"


sys.modules[__name__].__class__ = _ExportableModule
