
from pulumi.provider.experimental import component_provider_host
from builtin_info import BuiltinInfo

if __name__ == "__main__":
    component_provider_host([BuiltinInfo], "builtin-info-component", version="37.0.0")
