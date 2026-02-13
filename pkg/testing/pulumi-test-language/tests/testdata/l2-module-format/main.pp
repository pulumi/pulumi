// This tests that PCL allows both fully specified type tokens, and tokens that only specify the module and
// member name.

// First use the fully specified token to invoke and create a resource.
resource "res1" "module-format:mod_Resource:Resource" {
    text = invoke("module-format:mod_concatWorld:concatWorld", {
        value: "hello",
    }).result
}

output "out1" {
    value = call(res1, "call", { input = "x" }).output
}

// Next use just the module name as defined by the module format
resource "res2" "module-format:mod:Resource" {
    text = invoke("module-format:mod:concatWorld", {
        value: "goodbye",
    }).result
}

output "out2" {
    value = call(res2, "call", { input = "xx" }).output
}

// First use the fully specified token to invoke and create a resource.
resource "res3" "module-format:mod/nested_Resource:Resource" {
    text = invoke("module-format:mod/nested_concatWorld:concatWorld", {
        value: "hello",
    }).result
}

output "out3" {
    value = call(res3, "call", { input = "x" }).output
}

// Next use just the module name as defined by the module format
resource "res4" "module-format:mod/nested:Resource" {
    text = invoke("module-format:mod/nested:concatWorld", {
        value: "goodbye",
    }).result
}

output "out4" {
    value = call(res4, "call", { input = "xx" }).output
}