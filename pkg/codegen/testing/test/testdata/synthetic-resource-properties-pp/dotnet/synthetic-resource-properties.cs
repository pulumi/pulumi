using Pulumi;
using Synthetic = Pulumi.Synthetic.Synthetic;

class MyStack : Stack
{
    public MyStack()
    {
        var rt = new Synthetic.ResourceProperties.Root("rt", new Synthetic.ResourceProperties.RootArgs
        {
        });
        this.Trivial = rt;
        this.Simple = rt.Res1;
        this.Foo = rt.Res1.Apply(res1 => res1.Obj1?.Res2?.Obj2);
        this.Complex = rt.Res1.Apply(res1 => res1.Obj1?.Res2?.Obj2?.Answer);
    }

    [Output("trivial")]
    public Output<string> Trivial { get; set; }
    [Output("simple")]
    public Output<string> Simple { get; set; }
    [Output("foo")]
    public Output<string> Foo { get; set; }
    [Output("complex")]
    public Output<string> Complex { get; set; }
}
