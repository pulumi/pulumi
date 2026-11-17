resource "res1" "index-mod:indexMine:Resource" {
    text = invoke("index-mod:indexMine:concatWorld", {
        value: "hello",
    }).result
}

output "out1" {
    value = call(res1, "call", { input = "x" }).output
}

resource "res2" "index-mod:indexMine/nested:Resource" {
    text = invoke("index-mod:indexMine/nested:concatWorld", {
        value: "goodbye",
    }).result
}

output "out2" {
    value = call(res2, "call", { input = "xx" }).output
}
