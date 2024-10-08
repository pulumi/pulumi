resource "prov1" "pulumi:providers:testconfigprovider" {
    s = "x"
}

resource "prov1g" "testconfigprovider:index:ConfigGetter" {
  options {
    provider = prov1
  }
}
