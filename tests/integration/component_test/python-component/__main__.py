from pulumi.provider.experimental import Metadata, component_provider_host
from component import MyComponent

if __name__ == "__main__":
    component_provider_host(
        [MyComponent], Metadata(name="my-component", version="1.0.0")
    )
