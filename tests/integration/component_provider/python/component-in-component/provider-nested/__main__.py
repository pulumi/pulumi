from component import NestedComponent
from pulumi.provider.experimental import component_provider_host

if __name__ == "__main__":
    component_provider_host([NestedComponent], "provider-nested")
