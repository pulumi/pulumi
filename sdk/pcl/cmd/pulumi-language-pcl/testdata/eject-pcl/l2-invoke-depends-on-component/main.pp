resource "target" "component:index:ComponentCustomRefOutput" {
    value = "checked"
}

data = invoke("component:index:identity", { input = "reachable" }, {
    dependsOn = [target]
})

output "echoed" {
    value = data.result
}
