resource "res" "pkg:index:Resource" {
    value = "hello world"
}

package "pkg" {
    name = "parameterized"
    version = "1.3.7"
    parameterization = {
        version = "1.0.0"
        parameter = "cGtn"
    }
}