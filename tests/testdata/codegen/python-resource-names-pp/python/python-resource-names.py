import pulumi
import pulumi_azure_native as azure_native

cluster = azure_native.containerservice.ManagedCluster("cluster",
    agent_pool_profiles=[{
        "count": 2,
        "enable_fips": False,
        "kubelet_disk_type": azure_native.containerservice.KubeletDiskType.OS,
        "max_pods": 110,
        "mode": azure_native.containerservice.AgentPoolMode.SYSTEM,
        "name": "type1",
        "orchestrator_version": "1.21.9",
        "os_disk_size_gb": 128,
        "os_disk_type": azure_native.containerservice.OSDiskType.MANAGED,
        "os_sku": azure_native.containerservice.OSSKU.UBUNTU,
        "os_type": azure_native.containerservice.OSType.LINUX,
        "type": azure_native.containerservice.AgentPoolType.VIRTUAL_MACHINE_SCALE_SETS,
        "vm_size": "Standard_B2ms",
        "vnet_subnet_id": "/subscriptions/0282681f-7a9e-424b-80b2-96babd57a8a1/resourceGroups/test-rga2bd359a/providers/Microsoft.Network/virtualNetworks/test-vnet4b80e99b/subnets/test-subnet",
    }],
    dns_prefix="dns",
    enable_rbac=True,
    kubernetes_version="1.21.9",
    location="eastus",
    network_profile={
        "dns_service_ip": "10.10.0.10",
        "docker_bridge_cidr": "172.17.0.1/16",
        "load_balancer_profile": {
            "effective_outbound_ips": [{
                "id": "/subscriptions/0282681f-7a9e-424b-80b2-96babd57a8a1/resourceGroups/MC_test-rga2bd359a_test-aks5fb1e730_eastus/providers/Microsoft.Network/publicIPAddresses/2a2610b5-67f3-4aec-a277-a032b2364d70",
            }],
            "managed_outbound_ips": {
                "count": 1,
            },
        },
        "load_balancer_sku": "Standard",
        "network_plugin": azure_native.containerservice.NetworkPlugin.AZURE,
        "outbound_type": azure_native.containerservice.OutboundType.LOAD_BALANCER,
        "service_cidr": "10.10.0.0/16",
    },
    node_resource_group="MC_test-rga2bd359a_test-aks5fb1e730_eastus",
    resource_group_name="test-rga2bd359a",
    resource_name_="test-aks5fb1e730",
    service_principal_profile={
        "client_id": "64e3783f-3214-4ba7-bb52-12ad85412527",
    },
    sku={
        "name": azure_native.containerservice.ManagedClusterSKUName.BASIC,
        "tier": azure_native.containerservice.ManagedClusterSKUTier.FREE,
    },
    opts = pulumi.ResourceOptions(protect=True))
