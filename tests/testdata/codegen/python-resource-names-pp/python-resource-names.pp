resource cluster "azure-native:containerservice:ManagedCluster" {
    agentPoolProfiles =[{
            count = 2,
            enableFIPS = false,
            kubeletDiskType = "OS",
            maxPods = 110,
            mode = "System",
            name = "type1",
            orchestratorVersion = "1.21.9",
            osDiskSizeGB = 128,
            osDiskType = "Managed",
            osSKU = "Ubuntu",
            osType = "Linux",
            type = "VirtualMachineScaleSets",
            vmSize = "Standard_B2ms",
            vnetSubnetID = "/subscriptions/0282681f-7a9e-424b-80b2-96babd57a8a1/resourceGroups/test-rga2bd359a/providers/Microsoft.Network/virtualNetworks/test-vnet4b80e99b/subnets/test-subnet"
        }]
    dnsPrefix = "dns"
    enableRBAC = true
    kubernetesVersion = "1.21.9"
    location = "eastus"
    networkProfile ={
        dnsServiceIP = "10.10.0.10",
        dockerBridgeCidr = "172.17.0.1/16",
        loadBalancerProfile ={
            effectiveOutboundIPs =[{
                id = "/subscriptions/0282681f-7a9e-424b-80b2-96babd57a8a1/resourceGroups/MC_test-rga2bd359a_test-aks5fb1e730_eastus/providers/Microsoft.Network/publicIPAddresses/2a2610b5-67f3-4aec-a277-a032b2364d70"
                }],
                managedOutboundIPs ={
                    count = 1
                }
        },
        loadBalancerSku = "Standard",
        networkPlugin = "azure",
        outboundType = "loadBalancer",
        serviceCidr = "10.10.0.0/16"
    }
    nodeResourceGroup = "MC_test-rga2bd359a_test-aks5fb1e730_eastus"
    resourceGroupName = "test-rga2bd359a"
    resourceName = "test-aks5fb1e730"
    servicePrincipalProfile ={
        clientId = "64e3783f-3214-4ba7-bb52-12ad85412527"
    }
    sku ={
        name = "Basic",
        tier = "Free"
    }
    options {
        protect =true
    }
}
