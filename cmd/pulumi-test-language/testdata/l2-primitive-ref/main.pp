resource "res" "primitive-ref:index:Resource" {
    data = {
        boolean = false
        float = 2.17
        integer = -12
        string = "Goodbye"
        boolArray = [false, true]
        stringMap = {
            "two" = "turtle doves",
            "three" = "french hens",
        }
    }
}