module xovnoc

service Rack {
    // TODO: lambda code.
    // TODO: that big nasty UserData shell script.
    // TODO: possibly even refactor individual things into services (e.g., the networks).
    // TODO: we probably need a ToString()-like thing for services (e.g., ARN/ID for most AWS ones).

    resources {
        security := new rackSecurity {}
        network := new rackNetwork {
            existingVpc: existingVpc
            private: private
            privateApi: privateApi
            subnetCIDRs: subnetCIDRs
            subnetPrivateCIDRs: subnetPrivateCIDRs
            vpccidr: vpccidr
        }
        logging := new rackLogging {
            role: security.logSubscriptionFilterRole
        }
        storage := new rackStorage {}
        services := new rackServices {}
        volumes := new rackVolumes {
            vpc: network.vpc
            vpccidr: vpccidr
            subnets: private ? network.privateSubnets : subnets
        }
    }

    properties {
        // Amazon Machine Image: 
        // http://docs.aws.amazon.com/AmazonECS/latest/developerguide/launch_container_instance.html
        ami: string = ""
        // How much cpu should be reserved by the api web process.
        apiCpu: string = "128"
        // How much memory should be reserved by the api web process
        apiMemory: string = "128"
        // Autoscale rack instances
        autoscale: bool = false
        // How much cpu should be reserved by the builder
        buildCpu: string = "0"
        // Override the default build image
        buildImage: string = ""
        // How much memory should be reserved by the builder
        buildMemory: string = "1024"
        // Anonymous identifier
        clientId: string = "dev@xovnoc.com"
        // Default container disk size in GB
        containerDisk: number = 10
        // Development mode
        development: bool = false
        // Encrypt secrets with KMS
        encryption: bool = true
        // Existing VPC ID (if blank a VPC will be created)
        existingVpc: string = ""
        // Create applications that are only accessible inside the VPC
        internal: bool = false
        // A single line of shell script to run as CloudInit command early during instance boot.
        instanceBootCommand: string = ""
        // A single line of shell script to run as CloudInit command late during instance boot.
        instanceRunCommand: strign = ""
        // The number of instances in the runtime cluster
        instanceCount: number<3:> = 3
        // The type of the instances in the runtime cluster 
        instanceType: string = "t2.small"
        // The number of instances to update in a batch
        instanceUpdateBatchSize: number<1:> = 1
        // SSH key name for access to cluster instances
        key: string = ""
        // (REQUIRED) API HTTP password
        secret password: string<1, 50>
        // Create non publicly routable resources
        private: bool: false
        // Put Rack API Load Balancer in private network
        privateApi: bool: false
        // Public Subnet CIDR Blocks
        subnetCIDRs: string[] = [ "10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24" ]
        // Private Subnet CIDR Blocks
        subnetPrivateCIDRs: string = [ "10.0.4.0/24", "10.0.5.0/24", "10.0.6.0/24" ]
        // Default swap volume size in GB
        swapSize: number = 5
        // (REQUIRED) Xovnoc release version
        version: string<1:>
        // Default disk size in GB
        volumeSize: number = 50
        // VPC CIDR Block
        vpccidr: string = "10.0.0.0/16"
        // Dedicated Hardware
        tenancy: "default" | "dedicated" = "default"
    }
}

