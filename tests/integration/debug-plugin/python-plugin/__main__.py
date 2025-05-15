import pulumi
import pulumi.provider
import sys

class DebugProvider(pulumi.provider.Provider):
    schema = '{"name":"debugplugin","version":"0.0.1","resources":{"debugplugin:index:MyDebugResource":{}}}'
    def __init__(self):
        super().__init__("0.0.1", self.schema)


if __name__ == "__main__":
  pulumi.provider.main(DebugProvider(), sys.argv[1:])
