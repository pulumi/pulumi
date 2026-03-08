(callbacks)=
# Callbacks

The *callback system* allows programs, or even component libraries, to expose user-authored functions to engine and call them across process boundaries. These functions are defined alongside the rest of the Pulumi program, and run in the same process, but are typically not called directly by user code. Instead they are called by the engine at specific moments of the deployment. This mechanism enables features like [resource transforms](https://www.pulumi.com/docs/concepts/options/transforms/) and [resource hooks](https://www.pulumi.com/docs/iac/concepts/resources/options/hooks/).

The language SDKs start a gRPC server implementing the [](pulumirpc.Callbacks) service (for example, [_callbacks.py](gh-file:pulumi#sdk/python/lib/pulumi/runtime/_callbacks.py#L68)). This server exposes a single method: [](pulumirpc.Callbacks.Invoke). User-authored functions are registered with this server, and the engine is informed of their existence by calling [](pulumirpc.ResourceMonitor.RegisterStackTransform) or [](pulumirpc.ResourceMonitor.RegisterResourceHook).

The [](pulumirpc.Callback) message is used to register a callback. It represents a callback reference and contains two essential pieces of information:

* **target**: The gRPC address of the callback service (e.g., the gRPC server started by the language SDK).
* **token**: A unique identifier for the specific function to invoke within that service. Language SDKs typically use UUIDs to ensure uniqueness.

When the engine needs to invoke a callback, it calls [](pulumirpc.Callbacks.Invoke) with a [](pulumirpc.CallbackInvokeRequest). This message contains the token used to identify a function registered with an SDK's gRPC callback server.

* **token**  The token identifying which function to invoke.
* **request**: A serialized protobuf message containing the arguments. The specific message type depends on the callback type (e.g., [](pulumirpc.TransformRequest) for transforms).

The language host receives the request, looks up the function associated with the token, deserializes the request bytes, invokes the function, and returns a [](pulumirpc.CallbackInvokeResponse):

* **response**: A serialized protobuf message containing the result (e.g., [](pulumirpc.TransformResponse) for transforms).

The callback [](pulumirpc.CallbackInvokeRequest) and [](pulumirpc.CallbackInvokeResponse) are intentionally generic and untyped at the gRPC layer. This allows the system to support multiple callback types, each with their own request/response message formats serialized into the `bytes` fields.

The system supports multiple callback types, each with their own request/response message formats:

* **Resource transforms**: [](pulumirpc.TransformRequest) and [](pulumirpc.TransformResponse)
* **Invoke transforms**: [](pulumirpc.TransformInvokeRequest) and [](pulumirpc.TransformInvokeResponse)
* **Resource hooks**: [](pulumirpc.ResourceHookRequest) and [](pulumirpc.ResourceHookResponse)
