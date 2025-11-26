
from pulumi.provider.experimental import component_provider_host
from simple import Simple

if __name__ == "__main__":
    component_provider_host([Simple], "conformance-component", version="22.0.0")
