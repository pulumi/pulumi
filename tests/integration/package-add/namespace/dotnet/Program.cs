using Pulumi;
using MyNamespace.Mypkg;

return await Deployment.RunAsync(() =>
{
    var resource = new Resource("test");
});
