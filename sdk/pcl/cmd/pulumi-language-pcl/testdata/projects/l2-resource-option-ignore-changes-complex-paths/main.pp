resource "receiverIgnore" "nestedobject:index:Receiver" {
    details = [{
        key = "a"
        value = "b"
    }]
    options {
        ignoreChanges = [details[0].key]
    }
}

resource "mapIgnore" "nestedobject:index:MapContainer" {
    tags = {
        env = "prod"
    }
    options {
        ignoreChanges = [tags["env"]]
    }
}

resource "noIgnore" "nestedobject:index:Target" {
    name = "nothing"
}
