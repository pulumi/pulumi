"""Python component provider entry point."""
import sys
import pulumi
from pulumi.provider.experimental import component_provider_host
from component import Component

if __name__ == "__main__":
    component_provider_host([Component], "python-provider")
