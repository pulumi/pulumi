resource "first" "simple-invoke:index:StringResource" {
    text = "first"
}

resource "second" "simple-invoke:index:StringResource" {
    text = "second"
}

// getText fails unless a StringResource has already been created, so an SDK
// that drops the dependsOn option calls it during preview and fails the test.
gated = invoke("simple-invoke:index:getText", { text = "Goodbye" }, {
    dependsOn = [first]
})

// myInvoke fails when called with an unknown argument, so an SDK that does not
// await the gated invoke before chaining calls it during preview and fails the
// test.
chained = invoke("simple-invoke:index:myInvoke", { value = gated.result }, {
    dependsOn = [second]
})

output "result" {
    value = chained.result
}
