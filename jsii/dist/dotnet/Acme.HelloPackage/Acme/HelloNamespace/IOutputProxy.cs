using Amazon.JSII.Runtime.Deputy;

namespace Acme.HelloNamespace
{
    [JsiiTypeProxy(nativeType: typeof(IIOutput), fullyQualifiedName: "experiments.IOutput")]
    internal sealed class IOutputProxy : DeputyBase, IIOutput
    {
        private IOutputProxy(ByRefValue reference): base(reference)
        {
        }

        [JsiiMethod(name: "val", returnsJson: "{\"type\":{\"primitive\":\"any\"}}")]
        public object Val()
        {
            return InvokeInstanceMethod<object>(new object[]{});
        }
    }
}