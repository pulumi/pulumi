first = invoke("invokeresult:index:makeSettings", { displayName = "first", maxItems = 1 })

second = invoke("invokeresult:index:makeSettings", { displayName = "second", maxItems = 2 })

list = invoke("invokeresult:index:listOf", { items = [first, second] })

resource "holder" "invokeresult:index:Holder" {
    settings = first
    all = list.items
}
