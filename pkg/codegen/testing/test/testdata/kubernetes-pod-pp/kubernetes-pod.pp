resource bar "kubernetes:core/v1:Pod" {
    apiVersion = "v1"
    kind = "Pod"
    metadata = {
        namespace = "foo"
        name = "bar"
    }
    spec = {
        containers = [
            {
                name = "nginx"
                image = "nginx:1.14-alpine"
                ports = [{ containerPort = 80 }]
                resources = {
                    limits = {
                        memory = "20Mi"
                        cpu = 0.2
                    }
                }
            }
        ]
    }
}
