package {
  baseProviderName    = "extra-package-names"
  baseProviderVersion = "47.0.0"
}

resource "prov" "pulumi:providers:extra-package-names" {}

resource "viaProvider" "other:mod:Res" {
  choice = "first"
  obj = {
    label  = "explicit"
    choice = "second"
  }

  options {
    provider = prov
  }
}

resource "viaPackage" "other:mod:Res" {
  choice = "second"
  obj = {
    label  = "bare"
    choice = "first"
  }

  options {
    provider = extra-package-names
  }
}

thing = invoke("other:mod:getThing", { text = "hello" }, { provider = extra-package-names })

output "result" {
  value = thing.result
}
