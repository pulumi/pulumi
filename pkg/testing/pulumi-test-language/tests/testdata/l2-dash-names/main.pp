resource "first" "dash-names:dash-module:some-resource" {
    the-input = true
    nested-value = {
        nested-value = "nested"
    }
}

resource "third" "dash-names:dash-module:another-resource" {
    the-input = invoke("dash-names:dash-module:some-data", {
        the-input = first.the-output[0].nested-output
        entry-values = ["fuzz"]
    }).nested-output[0].entry-value
}

resource "trailing" "dash-names:dash-module:trailing-resource-" {
    trailing-input- = invoke("dash-names:dash-module:trailing-data-", {
        trailing-input- = "some-name-"
    }).trailing-output-
}
