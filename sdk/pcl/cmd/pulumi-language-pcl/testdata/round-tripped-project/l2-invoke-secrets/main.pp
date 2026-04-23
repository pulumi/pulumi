resource "res" "simple:index:Resource" {
    value = true
}

// inputs are plain and the invoke response is plain
output "nonSecret" {
    value = invoke("simple-invoke:index:secretInvoke", {
        value = "hello"
        secretResponse = false
    }).response
}

// referencing value from resource
// invoke response is secret => whole output is secret
output "firstSecret" {
    value = invoke("simple-invoke:index:secretInvoke", {
        value = "hello"
        secretResponse = res.value
    }).response
}

// inputs are secret, invoke response is plain => whole output is secret
output "secondSecret" {
    value = invoke("simple-invoke:index:secretInvoke", {
        value = secret("goodbye")
        secretResponse = false
    }).response
}

