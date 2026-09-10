resource "callable" "component:index:ComponentCallable" {
    value = "unused"
}

output "invokeResult" {
    value = invoke("simple-invoke:index:echoMap", {
        stringMap = {
            "my key"     = "one",
            "my.key"     = "two",
            "my-key"     = "three",
            "my_key"     = "four",
            "MY_KEY"     = "five",
            "myKey"      = "six",
            "__type"     = "seven",
            "__internal" = "eight",
        }
    }).stringMap
}

output "callResult" {
    value = call(callable, "echoMap", {
        stringMap = {
            "my key"     = "one",
            "my.key"     = "two",
            "my-key"     = "three",
            "my_key"     = "four",
            "MY_KEY"     = "five",
            "myKey"      = "six",
            "__type"     = "seven",
            "__internal" = "eight",
        }
    }).stringMap
}
