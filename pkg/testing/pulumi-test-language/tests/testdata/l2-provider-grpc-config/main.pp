# Cover interesting schema shapes.
resource "config_grpc_provider" "pulumi:providers:config-grpc" {

    string1 = ""
    string2 = "x"
    # Test a JSON-like string to see if it trips up JSON detectors.
    string3 = "{}"

    int1 = 0
    int2 = 42

    num1 = 0
    num2 = 42.42

    bool1 = true
    bool2 = false

    listString1 = []
    listString2 = ["", "foo"]
    listInt1 = [1, 2]

    mapString1 = {}
    mapString2 = {
       key1 = "value1"
       key2 = "value2"
    }
    mapInt1 = {
       key1 = 0
       key2 = 42
    }

    objString1 = {}
    objString2 = {
       x = "x-value"
    }
    objInt1 = {
       x = 42
    }
}

resource "config" "config-grpc:index:ConfigFetcher" {
  options {
    provider = config_grpc_provider
  }
}
