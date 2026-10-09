output "recovered" {
    value = recover(notImplemented("urn not available"), "recovered: ${error}")
}

resource "recovered_value" "simple:index:Resource" {
    value = recover(notImplemented("bool not available"), error != "")
}

resource "independent" "simple:index:Resource" {
    value = true
}
