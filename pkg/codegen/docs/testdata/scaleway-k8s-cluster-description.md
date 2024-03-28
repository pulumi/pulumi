Creates and manages Scaleway Kubernetes clusters. For more information, see [the documentation](https://developers.scaleway.com/en/products/k8s/api/).

## Example Usage

### Basic

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as scaleway from "@pulumiverse/scaleway";

const hedy = new scaleway.VpcPrivateNetwork("hedy", {});
const jack = new scaleway.KubernetesCluster("jack", {
    version: "1.24.3",
    cni: "cilium",
    privateNetworkId: hedy.id,
    deleteAdditionalResources: false,
});
const john = new scaleway.KubernetesNodePool("john", {
    clusterId: jack.id,
    nodeType: "DEV1-M",
    size: 1,
});
```
```python
import pulumi
import pulumiverse_scaleway as scaleway

hedy = scaleway.VpcPrivateNetwork("hedy")
jack = scaleway.KubernetesCluster("jack",
    version="1.24.3",
    cni="cilium",
    private_network_id=hedy.id,
    delete_additional_resources=False)
john = scaleway.KubernetesNodePool("john",
    cluster_id=jack.id,
    node_type="DEV1-M",
    size=1)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Scaleway = Pulumiverse.Scaleway;

return await Deployment.RunAsync(() => 
{
    var hedy = new Scaleway.VpcPrivateNetwork("hedy");

    var jack = new Scaleway.KubernetesCluster("jack", new()
    {
        Version = "1.24.3",
        Cni = "cilium",
        PrivateNetworkId = hedy.Id,
        DeleteAdditionalResources = false,
    });

    var john = new Scaleway.KubernetesNodePool("john", new()
    {
        ClusterId = jack.Id,
        NodeType = "DEV1-M",
        Size = 1,
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumiverse/pulumi-scaleway/sdk/go/scaleway"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hedy, err := scaleway.NewVpcPrivateNetwork(ctx, "hedy", nil)
		if err != nil {
			return err
		}
		jack, err := scaleway.NewKubernetesCluster(ctx, "jack", \u0026scaleway.KubernetesClusterArgs{
			Version:                   pulumi.String("1.24.3"),
			Cni:                       pulumi.String("cilium"),
			PrivateNetworkId:          hedy.ID(),
			DeleteAdditionalResources: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		_, err = scaleway.NewKubernetesNodePool(ctx, "john", \u0026scaleway.KubernetesNodePoolArgs{
			ClusterId: jack.ID(),
			NodeType:  pulumi.String("DEV1-M"),
			Size:      pulumi.Int(1),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.scaleway.VpcPrivateNetwork;
import com.pulumi.scaleway.KubernetesCluster;
import com.pulumi.scaleway.KubernetesClusterArgs;
import com.pulumi.scaleway.KubernetesNodePool;
import com.pulumi.scaleway.KubernetesNodePoolArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var hedy = new VpcPrivateNetwork("hedy");

        var jack = new KubernetesCluster("jack", KubernetesClusterArgs.builder()        
            .version("1.24.3")
            .cni("cilium")
            .privateNetworkId(hedy.id())
            .deleteAdditionalResources(false)
            .build());

        var john = new KubernetesNodePool("john", KubernetesNodePoolArgs.builder()        
            .clusterId(jack.id())
            .nodeType("DEV1-M")
            .size(1)
            .build());

    }
}
```
```yaml
resources:
  hedy:
    type: scaleway:VpcPrivateNetwork
  jack:
    type: scaleway:KubernetesCluster
    properties:
      version: 1.24.3
      cni: cilium
      privateNetworkId: ${hedy.id}
      deleteAdditionalResources: false
  john:
    type: scaleway:KubernetesNodePool
    properties:
      clusterId: ${jack.id}
      nodeType: DEV1-M
      size: 1
```
<!--End PulumiCodeChooser -->

### Multicloud

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as scaleway from "@pulumiverse/scaleway";

const henry = new scaleway.KubernetesCluster("henry", {
    type: "multicloud",
    version: "1.24.3",
    cni: "kilo",
    deleteAdditionalResources: false,
});
const friendFromOuterSpace = new scaleway.KubernetesNodePool("friendFromOuterSpace", {
    clusterId: henry.id,
    nodeType: "external",
    size: 0,
    minSize: 0,
});
```
```python
import pulumi
import pulumiverse_scaleway as scaleway

henry = scaleway.KubernetesCluster("henry",
    type="multicloud",
    version="1.24.3",
    cni="kilo",
    delete_additional_resources=False)
friend_from_outer_space = scaleway.KubernetesNodePool("friendFromOuterSpace",
    cluster_id=henry.id,
    node_type="external",
    size=0,
    min_size=0)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Scaleway = Pulumiverse.Scaleway;

return await Deployment.RunAsync(() => 
{
    var henry = new Scaleway.KubernetesCluster("henry", new()
    {
        Type = "multicloud",
        Version = "1.24.3",
        Cni = "kilo",
        DeleteAdditionalResources = false,
    });

    var friendFromOuterSpace = new Scaleway.KubernetesNodePool("friendFromOuterSpace", new()
    {
        ClusterId = henry.Id,
        NodeType = "external",
        Size = 0,
        MinSize = 0,
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumiverse/pulumi-scaleway/sdk/go/scaleway"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		henry, err := scaleway.NewKubernetesCluster(ctx, "henry", \u0026scaleway.KubernetesClusterArgs{
			Type:                      pulumi.String("multicloud"),
			Version:                   pulumi.String("1.24.3"),
			Cni:                       pulumi.String("kilo"),
			DeleteAdditionalResources: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}
		_, err = scaleway.NewKubernetesNodePool(ctx, "friendFromOuterSpace", \u0026scaleway.KubernetesNodePoolArgs{
			ClusterId: henry.ID(),
			NodeType:  pulumi.String("external"),
			Size:      pulumi.Int(0),
			MinSize:   pulumi.Int(0),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.scaleway.KubernetesCluster;
import com.pulumi.scaleway.KubernetesClusterArgs;
import com.pulumi.scaleway.KubernetesNodePool;
import com.pulumi.scaleway.KubernetesNodePoolArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var henry = new KubernetesCluster("henry", KubernetesClusterArgs.builder()        
            .type("multicloud")
            .version("1.24.3")
            .cni("kilo")
            .deleteAdditionalResources(false)
            .build());

        var friendFromOuterSpace = new KubernetesNodePool("friendFromOuterSpace", KubernetesNodePoolArgs.builder()        
            .clusterId(henry.id())
            .nodeType("external")
            .size(0)
            .minSize(0)
            .build());

    }
}
```
```yaml
resources:
  henry:
    type: scaleway:KubernetesCluster
    properties:
      type: multicloud
      version: 1.24.3
      cni: kilo
      deleteAdditionalResources: false
  friendFromOuterSpace:
    type: scaleway:KubernetesNodePool
    properties:
      clusterId: ${henry.id}
      nodeType: external
      size: 0
      minSize: 0
```
<!--End PulumiCodeChooser -->

For a detailed example of how to add or run Elastic Metal servers instead of instances on your cluster, please refer to this guide.

### With additional configuration

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as scaleway from "@pulumiverse/scaleway";

const hedy = new scaleway.VpcPrivateNetwork("hedy", {});
const johnKubernetesCluster = new scaleway.KubernetesCluster("johnKubernetesCluster", {
    description: "my awesome cluster",
    version: "1.24.3",
    cni: "calico",
    tags: [
        "i'm an awesome tag",
        "yay",
    ],
    privateNetworkId: hedy.id,
    deleteAdditionalResources: false,
    autoscalerConfig: {
        disableScaleDown: false,
        scaleDownDelayAfterAdd: "5m",
        estimator: "binpacking",
        expander: "random",
        ignoreDaemonsetsUtilization: true,
        balanceSimilarNodeGroups: true,
        expendablePodsPriorityCutoff: -5,
    },
});
const johnKubernetesNodePool = new scaleway.KubernetesNodePool("johnKubernetesNodePool", {
    clusterId: johnKubernetesCluster.id,
    nodeType: "DEV1-M",
    size: 3,
    autoscaling: true,
    autohealing: true,
    minSize: 1,
    maxSize: 5,
});
```
```python
import pulumi
import pulumiverse_scaleway as scaleway

hedy = scaleway.VpcPrivateNetwork("hedy")
john_kubernetes_cluster = scaleway.KubernetesCluster("johnKubernetesCluster",
    description="my awesome cluster",
    version="1.24.3",
    cni="calico",
    tags=[
        "i'm an awesome tag",
        "yay",
    ],
    private_network_id=hedy.id,
    delete_additional_resources=False,
    autoscaler_config=scaleway.KubernetesClusterAutoscalerConfigArgs(
        disable_scale_down=False,
        scale_down_delay_after_add="5m",
        estimator="binpacking",
        expander="random",
        ignore_daemonsets_utilization=True,
        balance_similar_node_groups=True,
        expendable_pods_priority_cutoff=-5,
    ))
john_kubernetes_node_pool = scaleway.KubernetesNodePool("johnKubernetesNodePool",
    cluster_id=john_kubernetes_cluster.id,
    node_type="DEV1-M",
    size=3,
    autoscaling=True,
    autohealing=True,
    min_size=1,
    max_size=5)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Scaleway = Pulumiverse.Scaleway;

return await Deployment.RunAsync(() => 
{
    var hedy = new Scaleway.VpcPrivateNetwork("hedy");

    var johnKubernetesCluster = new Scaleway.KubernetesCluster("johnKubernetesCluster", new()
    {
        Description = "my awesome cluster",
        Version = "1.24.3",
        Cni = "calico",
        Tags = new[]
        {
            "i'm an awesome tag",
            "yay",
        },
        PrivateNetworkId = hedy.Id,
        DeleteAdditionalResources = false,
        AutoscalerConfig = new Scaleway.Inputs.KubernetesClusterAutoscalerConfigArgs
        {
            DisableScaleDown = false,
            ScaleDownDelayAfterAdd = "5m",
            Estimator = "binpacking",
            Expander = "random",
            IgnoreDaemonsetsUtilization = true,
            BalanceSimilarNodeGroups = true,
            ExpendablePodsPriorityCutoff = -5,
        },
    });

    var johnKubernetesNodePool = new Scaleway.KubernetesNodePool("johnKubernetesNodePool", new()
    {
        ClusterId = johnKubernetesCluster.Id,
        NodeType = "DEV1-M",
        Size = 3,
        Autoscaling = true,
        Autohealing = true,
        MinSize = 1,
        MaxSize = 5,
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumiverse/pulumi-scaleway/sdk/go/scaleway"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		hedy, err := scaleway.NewVpcPrivateNetwork(ctx, "hedy", nil)
		if err != nil {
			return err
		}
		johnKubernetesCluster, err := scaleway.NewKubernetesCluster(ctx, "johnKubernetesCluster", \u0026scaleway.KubernetesClusterArgs{
			Description: pulumi.String("my awesome cluster"),
			Version:     pulumi.String("1.24.3"),
			Cni:         pulumi.String("calico"),
			Tags: pulumi.StringArray{
				pulumi.String("i'm an awesome tag"),
				pulumi.String("yay"),
			},
			PrivateNetworkId:          hedy.ID(),
			DeleteAdditionalResources: pulumi.Bool(false),
			AutoscalerConfig: \u0026scaleway.KubernetesClusterAutoscalerConfigArgs{
				DisableScaleDown:             pulumi.Bool(false),
				ScaleDownDelayAfterAdd:       pulumi.String("5m"),
				Estimator:                    pulumi.String("binpacking"),
				Expander:                     pulumi.String("random"),
				IgnoreDaemonsetsUtilization:  pulumi.Bool(true),
				BalanceSimilarNodeGroups:     pulumi.Bool(true),
				ExpendablePodsPriorityCutoff: -5,
			},
		})
		if err != nil {
			return err
		}
		_, err = scaleway.NewKubernetesNodePool(ctx, "johnKubernetesNodePool", \u0026scaleway.KubernetesNodePoolArgs{
			ClusterId:   johnKubernetesCluster.ID(),
			NodeType:    pulumi.String("DEV1-M"),
			Size:        pulumi.Int(3),
			Autoscaling: pulumi.Bool(true),
			Autohealing: pulumi.Bool(true),
			MinSize:     pulumi.Int(1),
			MaxSize:     pulumi.Int(5),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
<!--End PulumiCodeChooser -->

## Deprecation of default_pool

`default_pool` is deprecated in favour the `scaleway.KubernetesNodePool` resource. Here is a migration example.

Before:

<!--Start PulumiCodeChooser -->
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.scaleway.KubernetesCluster;
import com.pulumi.scaleway.KubernetesClusterArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var jack = new KubernetesCluster("jack", KubernetesClusterArgs.builder()        
            .cni("cilium")
            .defaultPool(%!v(PANIC=Format method: runtime error: invalid memory address or nil pointer dereference))
            .version("1.18.0")
            .build());

    }
}
```
```yaml
resources:
  jack:
    type: scaleway:KubernetesCluster
    properties:
      cni: cilium
      defaultPool:
        - nodeType: DEV1-M
          size: 1
      version: 1.18.0
```
<!--End PulumiCodeChooser -->

After:

<!--Start PulumiCodeChooser -->
```typescript
import * as pulumi from "@pulumi/pulumi";
import * as scaleway from "@pulumiverse/scaleway";

const jack = new scaleway.KubernetesCluster("jack", {
    version: "1.18.0",
    cni: "cilium",
});
const _default = new scaleway.KubernetesNodePool("default", {
    clusterId: jack.id,
    nodeType: "DEV1-M",
    size: 1,
});
```
```python
import pulumi
import pulumiverse_scaleway as scaleway

jack = scaleway.KubernetesCluster("jack",
    version="1.18.0",
    cni="cilium")
default = scaleway.KubernetesNodePool("default",
    cluster_id=jack.id,
    node_type="DEV1-M",
    size=1)
```
```csharp
using System.Collections.Generic;
using System.Linq;
using Pulumi;
using Scaleway = Pulumiverse.Scaleway;

return await Deployment.RunAsync(() => 
{
    var jack = new Scaleway.KubernetesCluster("jack", new()
    {
        Version = "1.18.0",
        Cni = "cilium",
    });

    var @default = new Scaleway.KubernetesNodePool("default", new()
    {
        ClusterId = jack.Id,
        NodeType = "DEV1-M",
        Size = 1,
    });

});
```
```go
package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumiverse/pulumi-scaleway/sdk/go/scaleway"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		jack, err := scaleway.NewKubernetesCluster(ctx, "jack", \u0026scaleway.KubernetesClusterArgs{
			Version: pulumi.String("1.18.0"),
			Cni:     pulumi.String("cilium"),
		})
		if err != nil {
			return err
		}
		_, err = scaleway.NewKubernetesNodePool(ctx, "default", \u0026scaleway.KubernetesNodePoolArgs{
			ClusterId: jack.ID(),
			NodeType:  pulumi.String("DEV1-M"),
			Size:      pulumi.Int(1),
		})
		if err != nil {
			return err
		}
		return nil
	})
}
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.pulumi.core.Output;
import com.pulumi.scaleway.KubernetesCluster;
import com.pulumi.scaleway.KubernetesClusterArgs;
import com.pulumi.scaleway.KubernetesNodePool;
import com.pulumi.scaleway.KubernetesNodePoolArgs;
import java.util.List;
import java.util.ArrayList;
import java.util.Map;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Paths;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        var jack = new KubernetesCluster("jack", KubernetesClusterArgs.builder()        
            .version("1.18.0")
            .cni("cilium")
            .build());

        var default_ = new KubernetesNodePool("default", KubernetesNodePoolArgs.builder()        
            .clusterId(jack.id())
            .nodeType("DEV1-M")
            .size(1)
            .build());

    }
}
```
```yaml
resources:
  jack:
    type: scaleway:KubernetesCluster
    properties:
      version: 1.18.0
      cni: cilium
  default:
    type: scaleway:KubernetesNodePool
    properties:
      clusterId: ${jack.id}
      nodeType: DEV1-M
      size: 1
```
<!--End PulumiCodeChooser -->

Once you have moved all the `default_pool` into their own object, you will need to import them. If your pool had the ID 11111111-1111-1111-1111-111111111111 in the `fr-par` region, you can import it by typing:

```bash
$ terraform import scaleway_k8s_pool.default fr-par/11111111-1111-1111-1111-111111111111
```

Then you will only need to type `pulumi up` to have a smooth migration.

## Import

Kubernetes clusters can be imported using the `{region}/{id}`, e.g.

bash

```sh
$ pulumi import scaleway:index/kubernetesCluster:KubernetesCluster mycluster fr-par/11111111-1111-1111-1111-111111111111
```

