// "null" and "map" are currently commented out due to
// https://github.com/pulumi/pulumi/issues/19015.

//output "null" {
//    value = null
//}

output "array" {
    value = [null]
}

//output "map" {
//    value = {
//        "key": null
//    }
//}