using Amazon.JSII.Runtime.Deputy;

namespace Acme.HelloNamespace
{
    [JsiiClass(nativeType: typeof(HelloJsii), fullyQualifiedName: "experiments.HelloJsii")]
    public class HelloJsii : DeputyBase
    {
        public HelloJsii(): base(new DeputyProps(new object[]{}))
        {
        }

        protected HelloJsii(ByRefValue reference): base(reference)
        {
        }

        protected HelloJsii(DeputyProps props): base(props)
        {
        }

        [JsiiProperty(name: "strings", typeJson: "{\"collection\":{\"kind\":\"array\",\"elementtype\":{\"primitive\":\"string\"}}}")]
        public virtual string[] Strings
        {
            get => GetInstanceProperty<string[]>();
            set => SetInstanceProperty(value);
        }

        [JsiiMethod(name: "baz", returnsJson: "{\"type\":{\"union\":{\"types\":[{\"primitive\":\"string\"},{\"fqn\":\"experiments.IOutput\"}]}}}", parametersJson: "[{\"name\":\"input\",\"type\":{\"union\":{\"types\":[{\"primitive\":\"number\"},{\"fqn\":\"experiments.IOutput\"}]}}}]")]
        public virtual object Baz(object input)
        {
            return InvokeInstanceMethod<object>(new object[]{input});
        }
    }
}