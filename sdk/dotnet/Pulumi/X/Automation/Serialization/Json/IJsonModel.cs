namespace Pulumi.X.Automation.Serialization.Json
{
    internal interface IJsonModel<out T>
    {
        T Convert();
    }
}
