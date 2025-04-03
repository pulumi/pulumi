resource "baseCustomDefault" "extension:index:Custom" {
  value = "baseCustomDefault"
}

resource "baseExplicit" "pulumi:providers:extension" {
  value = "baseExplicit"
}

resource "baseCustomExplicit" "extension:index:Custom" {
  value = "baseCustomExplicit"
  options {
    provider = baseExplicit
  }
}

package "replaced" {
  baseProviderName = "extension"
  baseProviderVersion = "17.17.17"
  replacement {
    name = "replaced"
    version = "34.34.34"
    vaue = "cmVwbGFjZWQtcGFyYW1ldGVyCg==" // base64(utf8_bytes("replaced-parameter"))
  }
}

resource "replacedCustomDefault" "replaced:index:Custom" {
  value = "replacedCustomDefault"
}

resource "replacedExplicit" "pulumi:providers:replaced" {
  value = "replacedExplicit"
}

resource "replacedCustomExplicit" "replaced:index:Custom" {
  value = "replacedCustomExplicit"
  options {
    provider = replacedExplicit
  }
}

package "extended1" {
  baseProviderName = "extension"
  baseProviderVersion = "17.17.17"
  replacement {
    name = "extended1"
    version = "51.51.51"
    value = "ZXh0ZW5kZWQxLXBhcmFtZXRlcgo=" // base64(utf8_bytes("extended1-parameter"))
  }
}

resource "extended1CustomDefault" "extended1:index:Custom" {
  value = "extended1CustomDefault"
}

resource "extended1CustomExplicit" "extended1:index:Custom" {
  value = "extended1CustomExplicit"
  options {
    provider = baseExplicit
  }
}

package "extended2" {
  baseProviderName = "extension"
  baseProviderVersion = "17.17.17"
  replacement {
    name = "extended2"
    version = "68.68.68"
    value = "ZXh0ZW5kZWQyLXBhcmFtZXRlcgo=" // base64(utf8_bytes("extended2-parameter"))
  }
}

resource "extended2CustomDefault" "extended2:index:Custom" {
  value = "extended2CustomDefault"
}

resource "extended2CustomExplicit" "extended2:index:Custom" {
  value = "extended2CustomExplicit"
  options {
    provider = baseExplicit
  }
}
