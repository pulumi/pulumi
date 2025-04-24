import pulumi
import pulumi_kubernetes as kubernetes

pulumi_kubernetes_operator_deployment = kubernetes.apps.v1.Deployment("pulumi_kubernetes_operatorDeployment",
    api_version="apps/v1",
    kind="Deployment",
    metadata={
        "name": "pulumi-kubernetes-operator",
    },
    spec={
        "replicas": 1,
        "selector": {
            "match_labels": {
                "name": "pulumi-kubernetes-operator",
            },
        },
        "template": {
            "metadata": {
                "labels": {
                    "name": "pulumi-kubernetes-operator",
                },
            },
            "spec": {
                "service_account_name": "pulumi-kubernetes-operator",
                "image_pull_secrets": [{
                    "name": "pulumi-kubernetes-operator",
                }],
                "containers": [{
                    "name": "pulumi-kubernetes-operator",
                    "image": "pulumi/pulumi-kubernetes-operator:v0.0.2",
                    "command": ["pulumi-kubernetes-operator"],
                    "args": ["--zap-level=debug"],
                    "image_pull_policy": "Always",
                    "env": [
                        {
                            "name": "WATCH_NAMESPACE",
                            "value_from": {
                                "field_ref": {
                                    "field_path": "metadata.namespace",
                                },
                            },
                        },
                        {
                            "name": "POD_NAME",
                            "value_from": {
                                "field_ref": {
                                    "field_path": "metadata.name",
                                },
                            },
                        },
                        {
                            "name": "OPERATOR_NAME",
                            "value": "pulumi-kubernetes-operator",
                        },
                    ],
                }],
            },
        },
    })
pulumi_kubernetes_operator_role = kubernetes.rbac.v1.Role("pulumi_kubernetes_operatorRole",
    api_version="rbac.authorization.k8s.io/v1",
    kind="Role",
    metadata={
        "creation_timestamp": None,
        "name": "pulumi-kubernetes-operator",
    },
    rules=[
        {
            "api_groups": [""],
            "resources": [
                "pods",
                "services",
                "services/finalizers",
                "endpoints",
                "persistentvolumeclaims",
                "events",
                "configmaps",
                "secrets",
            ],
            "verbs": [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            "api_groups": ["apps"],
            "resources": [
                "deployments",
                "daemonsets",
                "replicasets",
                "statefulsets",
            ],
            "verbs": [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
        {
            "api_groups": ["monitoring.coreos.com"],
            "resources": ["servicemonitors"],
            "verbs": [
                "get",
                "create",
            ],
        },
        {
            "api_groups": ["apps"],
            "resource_names": ["pulumi-kubernetes-operator"],
            "resources": ["deployments/finalizers"],
            "verbs": ["update"],
        },
        {
            "api_groups": [""],
            "resources": ["pods"],
            "verbs": ["get"],
        },
        {
            "api_groups": ["apps"],
            "resources": [
                "replicasets",
                "deployments",
            ],
            "verbs": ["get"],
        },
        {
            "api_groups": ["pulumi.com"],
            "resources": ["*"],
            "verbs": [
                "create",
                "delete",
                "get",
                "list",
                "patch",
                "update",
                "watch",
            ],
        },
    ])
pulumi_kubernetes_operator_role_binding = kubernetes.rbac.v1.RoleBinding("pulumi_kubernetes_operatorRoleBinding",
    kind="RoleBinding",
    api_version="rbac.authorization.k8s.io/v1",
    metadata={
        "name": "pulumi-kubernetes-operator",
    },
    subjects=[{
        "kind": "ServiceAccount",
        "name": "pulumi-kubernetes-operator",
    }],
    role_ref={
        "kind": "Role",
        "name": "pulumi-kubernetes-operator",
        "api_group": "rbac.authorization.k8s.io",
    })
pulumi_kubernetes_operator_service_account = kubernetes.core.v1.ServiceAccount("pulumi_kubernetes_operatorServiceAccount",
    api_version="v1",
    kind="ServiceAccount",
    metadata={
        "name": "pulumi-kubernetes-operator",
    })
