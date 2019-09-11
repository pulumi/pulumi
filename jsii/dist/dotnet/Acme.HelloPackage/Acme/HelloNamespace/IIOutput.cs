using Amazon.JSII.Runtime.Deputy;

namespace Acme.HelloNamespace
{
    [JsiiInterface(nativeType: typeof(IIOutput), fullyQualifiedName: "experiments.IOutput")]
    public interface IIOutput
    {
        [JsiiMethod(name: "val", returnsJson: "{\"type\":{\"primitive\":\"any\"}}")]
        object Val();
    }
}