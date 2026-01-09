// properties field is bound to union case ServerPropertiesForReplica
resource replica "basic-unions:index:ExampleServer" {
    properties = {
        createMode = "Replica"
        version = "0.1.0-dev"
    }
}

// properties field is bound to union case ServerPropertiesForRestore
resource restore "basic-unions:index:ExampleServer" {
    properties = {
        createMode = "PointInTimeRestore"
        restorePointInTime = "example"
    }
}
