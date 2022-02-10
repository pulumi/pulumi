import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

export = async () => {
    // VPC
    const eksVpc = new aws.ec2.Vpc("eksVpc", {
        cidrBlock: "10.100.0.0/16",
        instanceTenancy: "default",
        enableDnsHostnames: true,
        enableDnsSupport: true,
        tags: {
            Name: "pulumi-eks-vpc",
        },
    });
    const eksIgw = new aws.ec2.InternetGateway("eksIgw", {
        vpcId: eksVpc.id,
        tags: {
            Name: "pulumi-vpc-ig",
        },
    });
    const eksRouteTable = new aws.ec2.RouteTable("eksRouteTable", {
        vpcId: eksVpc.id,
        routes: [{
            cidrBlock: "0.0.0.0/0",
            gatewayId: eksIgw.id,
        }],
        tags: {
            Name: "pulumi-vpc-rt",
        },
    });
    // Subnets, one for each AZ in a region
    const zones = await aws.getAvailabilityZones({});
    const vpcSubnet: aws.ec2.Subnet[];
    for (const range of zones.names.map((k, v) => {key: k, value: v})) {
        vpcSubnet.push(new aws.ec2.Subnet(`vpcSubnet-${range.key}`, {
            assignIpv6AddressOnCreation: false,
            vpcId: eksVpc.id,
            mapPublicIpOnLaunch: true,
            cidrBlock: `10.100.${range.key}.0/24`,
            availabilityZone: range.value,
            tags: {
                Name: `pulumi-sn-${range.value}`,
            },
        }));
    }
    const rta: aws.ec2.RouteTableAssociation[];
    for (const range of zones.names.map((k, v) => {key: k, value: v})) {
        rta.push(new aws.ec2.RouteTableAssociation(`rta-${range.key}`, {
            routeTableId: eksRouteTable.id,
            subnetId: vpcSubnet[range.key].id,
        }));
    }
    const subnetIds = vpcSubnet.map(__item => __item.id);
    const eksSecurityGroup = new aws.ec2.SecurityGroup("eksSecurityGroup", {
        vpcId: eksVpc.id,
        description: "Allow all HTTP(s) traffic to EKS Cluster",
        tags: {
            Name: "pulumi-cluster-sg",
        },
        ingress: [
            {
                cidrBlocks: ["0.0.0.0/0"],
                fromPort: 443,
                toPort: 443,
                protocol: "tcp",
                description: "Allow pods to communicate with the cluster API Server.",
            },
            {
                cidrBlocks: ["0.0.0.0/0"],
                fromPort: 80,
                toPort: 80,
                protocol: "tcp",
                description: "Allow internet access to pods",
            },
        ],
    });
    // EKS Cluster Role
    const eksRole = new aws.iam.Role("eksRole", {assumeRolePolicy: JSON.stringify({
        Version: "2012-10-17",
        Statement: [{
            Action: "sts:AssumeRole",
            Principal: {
                Service: "eks.amazonaws.com",
            },
            Effect: "Allow",
            Sid: "",
        }],
    })});
    const servicePolicyAttachment = new aws.iam.RolePolicyAttachment("servicePolicyAttachment", {
        role: eksRole.id,
        policyArn: "arn:aws:iam::aws:policy/AmazonEKSServicePolicy",
    });
    const clusterPolicyAttachment = new aws.iam.RolePolicyAttachment("clusterPolicyAttachment", {
        role: eksRole.id,
        policyArn: "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy",
    });
    // EC2 NodeGroup Role
    const ec2Role = new aws.iam.Role("ec2Role", {assumeRolePolicy: JSON.stringify({
        Version: "2012-10-17",
        Statement: [{
            Action: "sts:AssumeRole",
            Principal: {
                Service: "ec2.amazonaws.com",
            },
            Effect: "Allow",
            Sid: "",
        }],
    })});
    const workerNodePolicyAttachment = new aws.iam.RolePolicyAttachment("workerNodePolicyAttachment", {
        role: ec2Role.id,
        policyArn: "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    });
    const cniPolicyAttachment = new aws.iam.RolePolicyAttachment("cniPolicyAttachment", {
        role: ec2Role.id,
        policyArn: "arn:aws:iam::aws:policy/AmazonEKSCNIPolicy",
    });
    const registryPolicyAttachment = new aws.iam.RolePolicyAttachment("registryPolicyAttachment", {
        role: ec2Role.id,
        policyArn: "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
    });
    // EKS Cluster
    const eksCluster = new aws.eks.Cluster("eksCluster", {
        roleArn: eksRole.arn,
        tags: {
            Name: "pulumi-eks-cluster",
        },
        vpcConfig: {
            publicAccessCidrs: ["0.0.0.0/0"],
            securityGroupIds: [eksSecurityGroup.id],
            subnetIds: subnetIds,
        },
    });
    const nodeGroup = new aws.eks.NodeGroup("nodeGroup", {
        clusterName: eksCluster.name,
        nodeGroupName: "pulumi-eks-nodegroup",
        nodeRoleArn: ec2Role.arn,
        subnetIds: subnetIds,
        tags: {
            Name: "pulumi-cluster-nodeGroup",
        },
        scalingConfig: {
            desiredSize: 2,
            maxSize: 2,
            minSize: 1,
        },
    });
    const clusterName = eksCluster.name;
    const kubeconfig = pulumi.all([eksCluster.endpoint, eksCluster.certificateAuthority, eksCluster.name]).apply(([endpoint, certificateAuthority, name]) => JSON.stringify({
        apiVersion: "v1",
        clusters: [{
            cluster: {
                server: endpoint,
                "certificate-authority-data": certificateAuthority.data,
            },
            name: "kubernetes",
        }],
        contexts: [{
            contest: {
                cluster: "kubernetes",
                user: "aws",
            },
        }],
        "current-context": "aws",
        kind: "Config",
        users: [{
            name: "aws",
            user: {
                exec: {
                    apiVersion: "client.authentication.k8s.io/v1alpha1",
                    command: "aws-iam-authenticator",
                },
                args: [
                    "token",
                    "-i",
                    name,
                ],
            },
        }],
    }));
    return {
        clusterName: clusterName,
        kubeconfig: kubeconfig,
    };
}
