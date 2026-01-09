using System.Collections.Generic;
using System.Linq;
using System.Text.Json;
using System.Threading.Tasks;
using Pulumi;
using Aws = Pulumi.Aws;

return await Deployment.RunAsync(async() => 
{
    // VPC
    var eksVpc = new Aws.Ec2.Vpc("eksVpc", new()
    {
        CidrBlock = "10.100.0.0/16",
        InstanceTenancy = "default",
        EnableDnsHostnames = true,
        EnableDnsSupport = true,
        Tags = 
        {
            { "Name", "pulumi-eks-vpc" },
        },
    });

    var eksIgw = new Aws.Ec2.InternetGateway("eksIgw", new()
    {
        VpcId = eksVpc.Id,
        Tags = 
        {
            { "Name", "pulumi-vpc-ig" },
        },
    });

    var eksRouteTable = new Aws.Ec2.RouteTable("eksRouteTable", new()
    {
        VpcId = eksVpc.Id,
        Routes = new[]
        {
            new Aws.Ec2.Inputs.RouteTableRouteArgs
            {
                CidrBlock = "0.0.0.0/0",
                GatewayId = eksIgw.Id,
            },
        },
        Tags = 
        {
            { "Name", "pulumi-vpc-rt" },
        },
    });

    // Subnets, one for each AZ in a region
    var zones = await Aws.GetAvailabilityZones.InvokeAsync();

    var vpcSubnet = new List<Aws.Ec2.Subnet>();
    foreach (var range in zones.Names.Select((v, k) => new { Key = k, Value = v }))
    {
        vpcSubnet.Add(new Aws.Ec2.Subnet($"vpcSubnet-{range.Key}", new()
        {
            AssignIpv6AddressOnCreation = false,
            VpcId = eksVpc.Id,
            MapPublicIpOnLaunch = true,
            CidrBlock = $"10.100.{range.Key}.0/24",
            AvailabilityZone = range.Value,
            Tags = 
            {
                { "Name", $"pulumi-sn-{range.Value}" },
            },
        }));
    }
    var rta = new List<Aws.Ec2.RouteTableAssociation>();
    foreach (var range in zones.Names.Select((v, k) => new { Key = k, Value = v }))
    {
        rta.Add(new Aws.Ec2.RouteTableAssociation($"rta-{range.Key}", new()
        {
            RouteTableId = eksRouteTable.Id,
            SubnetId = vpcSubnet[range.Key].Id,
        }));
    }
    var subnetIds = vpcSubnet.Select(__item => __item.Id).ToList();

    var eksSecurityGroup = new Aws.Ec2.SecurityGroup("eksSecurityGroup", new()
    {
        VpcId = eksVpc.Id,
        Description = "Allow all HTTP(s) traffic to EKS Cluster",
        Tags = 
        {
            { "Name", "pulumi-cluster-sg" },
        },
        Ingress = new[]
        {
            new Aws.Ec2.Inputs.SecurityGroupIngressArgs
            {
                CidrBlocks = new[]
                {
                    "0.0.0.0/0",
                },
                FromPort = 443,
                ToPort = 443,
                Protocol = "tcp",
                Description = "Allow pods to communicate with the cluster API Server.",
            },
            new Aws.Ec2.Inputs.SecurityGroupIngressArgs
            {
                CidrBlocks = new[]
                {
                    "0.0.0.0/0",
                },
                FromPort = 80,
                ToPort = 80,
                Protocol = "tcp",
                Description = "Allow internet access to pods",
            },
        },
    });

    // EKS Cluster Role
    var eksRole = new Aws.Iam.Role("eksRole", new()
    {
        AssumeRolePolicy = JsonSerializer.Serialize(new Dictionary<string, object?>
        {
            ["Version"] = "2012-10-17",
            ["Statement"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["Action"] = "sts:AssumeRole",
                    ["Principal"] = new Dictionary<string, object?>
                    {
                        ["Service"] = "eks.amazonaws.com",
                    },
                    ["Effect"] = "Allow",
                    ["Sid"] = "",
                },
            },
        }),
    });

    var servicePolicyAttachment = new Aws.Iam.RolePolicyAttachment("servicePolicyAttachment", new()
    {
        Role = eksRole.Id,
        PolicyArn = "arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
    });

    var clusterPolicyAttachment = new Aws.Iam.RolePolicyAttachment("clusterPolicyAttachment", new()
    {
        Role = eksRole.Id,
        PolicyArn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
    });

    // EC2 NodeGroup Role
    var ec2Role = new Aws.Iam.Role("ec2Role", new()
    {
        AssumeRolePolicy = JsonSerializer.Serialize(new Dictionary<string, object?>
        {
            ["Version"] = "2012-10-17",
            ["Statement"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["Action"] = "sts:AssumeRole",
                    ["Principal"] = new Dictionary<string, object?>
                    {
                        ["Service"] = "ec2.amazonaws.com",
                    },
                    ["Effect"] = "Allow",
                    ["Sid"] = "",
                },
            },
        }),
    });

    var workerNodePolicyAttachment = new Aws.Iam.RolePolicyAttachment("workerNodePolicyAttachment", new()
    {
        Role = ec2Role.Id,
        PolicyArn = "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    });

    var cniPolicyAttachment = new Aws.Iam.RolePolicyAttachment("cniPolicyAttachment", new()
    {
        Role = ec2Role.Id,
        PolicyArn = "arn:aws:iam::aws:policy/AmazonEKSCNIPolicy",
    });

    var registryPolicyAttachment = new Aws.Iam.RolePolicyAttachment("registryPolicyAttachment", new()
    {
        Role = ec2Role.Id,
        PolicyArn = "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
    });

    // EKS Cluster
    var eksCluster = new Aws.Eks.Cluster("eksCluster", new()
    {
        RoleArn = eksRole.Arn,
        Tags = 
        {
            { "Name", "pulumi-eks-cluster" },
        },
        VpcConfig = new Aws.Eks.Inputs.ClusterVpcConfigArgs
        {
            PublicAccessCidrs = new[]
            {
                "0.0.0.0/0",
            },
            SecurityGroupIds = new[]
            {
                eksSecurityGroup.Id,
            },
            SubnetIds = subnetIds,
        },
    });

    var nodeGroup = new Aws.Eks.NodeGroup("nodeGroup", new()
    {
        ClusterName = eksCluster.Name,
        NodeGroupName = "pulumi-eks-nodegroup",
        NodeRoleArn = ec2Role.Arn,
        SubnetIds = subnetIds,
        Tags = 
        {
            { "Name", "pulumi-cluster-nodeGroup" },
        },
        ScalingConfig = new Aws.Eks.Inputs.NodeGroupScalingConfigArgs
        {
            DesiredSize = 2,
            MaxSize = 2,
            MinSize = 1,
        },
    });

    return new Dictionary<string, object?>
    {
        ["clusterName"] = eksCluster.Name,
        ["kubeconfig"] = Output.JsonSerialize(Output.Create(new Dictionary<string, object?>
        {
            ["apiVersion"] = "v1",
            ["clusters"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["cluster"] = new Dictionary<string, object?>
                    {
                        ["server"] = eksCluster.Endpoint,
                        ["certificate-authority-data"] = eksCluster.CertificateAuthority.Apply(certificateAuthority => certificateAuthority.Data),
                    },
                    ["name"] = "kubernetes",
                },
            },
            ["contexts"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["contest"] = new Dictionary<string, object?>
                    {
                        ["cluster"] = "kubernetes",
                        ["user"] = "aws",
                    },
                },
            },
            ["current-context"] = "aws",
            ["kind"] = "Config",
            ["users"] = new[]
            {
                new Dictionary<string, object?>
                {
                    ["name"] = "aws",
                    ["user"] = new Dictionary<string, object?>
                    {
                        ["exec"] = new Dictionary<string, object?>
                        {
                            ["apiVersion"] = "client.authentication.k8s.io/v1alpha1",
                            ["command"] = "aws-iam-authenticator",
                        },
                        ["args"] = new object?[]
                        {
                            "token",
                            "-i",
                            eksCluster.Name,
                        },
                    },
                },
            },
        })),
    };
});

