# This provider covers scenarios where user passes secret values to the provider.
resource "config_grpc_provider" "pulumi:providers:config-grpc" {
    string1 = invoke("config-grpc:index:toSecret", {string1 = "SECRET"}).string1
    int1 = invoke("config-grpc:index:toSecret", {int1 = 1234567890}).int1
    num1 = invoke("config-grpc:index:toSecret", {num1 = 123456.7890}).num1
    bool1 = invoke("config-grpc:index:toSecret", {bool1 = true}).bool1

    listString1 = invoke("config-grpc:index:toSecret", {listString1 = ["SECRET", "SECRET2"]}).listString1
    listString2 = ["VALUE", invoke("config-grpc:index:toSecret", {string1 = "SECRET"}).string1]

    # TODO[pulumi/pulumi#17535] this currently breaks Go compilation unfortunately.
    # mapString1 = invoke("config-grpc:index:toSecret", {mapString1 = { key1 = "SECRET", key2 = "SECRET2" }}).mapString1

    mapString2 = {
       key1 = "value1"
       key2 = invoke("config-grpc:index:toSecret", {string1 = "SECRET"}).string1
    }

    # TODO[pulumi/pulumi#17535] this breaks Go compilation as well.
    # os1 = invoke("config-grpc:index:toSecret", {objString1 = { x = "SECRET" }}).objString1

    objString2 = { x = invoke("config-grpc:index:toSecret", {string1 = "SECRET"}).string1 }
}

resource "config" "config-grpc:index:ConfigFetcher" {
  options {
    provider = config_grpc_provider
  }
}
