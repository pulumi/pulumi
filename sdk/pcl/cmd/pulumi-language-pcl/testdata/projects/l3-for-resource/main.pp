resource "source" "nestedobject:index:Container" {
    inputs = ["a", "b", "c"]
}

# for over list<object> output
resource "receiver" "nestedobject:index:Receiver" {
    details = [for _, detail in source.details : {
        key = detail.key
        value = detail.value
    }]
}

# for over list<string> output
resource "fromSimple" "nestedobject:index:Container" {
    inputs = [for _, detail in source.details : detail.value]
}

# for producing a map
resource "mapped" "nestedobject:index:MapContainer" {
    tags = {for _, detail in source.details : detail.key => detail.value}
}
