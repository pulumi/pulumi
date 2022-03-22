using Pulumi;
using Aws = Pulumi.Aws;

class MyStack : Stack
{
    public MyStack()
    {
        var webServer = new Aws.Ec2.Instance("webServer", new Aws.Ec2.InstanceArgs
        {
            Ami = Output.Create(Aws.GetAmi.InvokeAsync(new Aws.GetAmiArgs
            {
                %!v(PANIC=Format method: interface conversion: model.Expression is *model.TemplateExpression, not *model.LiteralValueExpression))).Apply(invoke => invoke.Id),
            });
        }

}
