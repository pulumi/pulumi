resource "child" "inheritabstract:index:ConcreteChild" {
    seed = "s"
    extra = "e"
}

output "abstractOutput" {
    value = child.abstractOutput
}

output "concreteOutput" {
    value = child.concreteOutput
}
