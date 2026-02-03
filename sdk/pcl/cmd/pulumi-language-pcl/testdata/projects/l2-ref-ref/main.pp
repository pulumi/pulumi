resource "res" "ref-ref:index:Resource" {
    data = {
        innerData = {
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
        boolean = true
        float = 4.5
        integer = 1024
        string = "Hello"
        boolArray = []
        stringMap = {
            "x" = "100"
            "y" = "200"
        }
    }
}