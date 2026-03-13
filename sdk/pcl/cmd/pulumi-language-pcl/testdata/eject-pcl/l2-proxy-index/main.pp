# Check we can index into properties of objects returned in outputs, this is similar to ref-ref but 
# we index into the outputs

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
        boolArray = [true]
        stringMap = {
            "x" = "100"
            "y" = "200"
        }
    }
}

output "bool" {
  value = res.data.boolean
}

output "array" { 
  value = res.data.boolArray[0]
}

output "map" {  
  value = res.data.stringMap["x"]
}

output "nested" {
  value = res.data.innerData.stringMap["three"]
}