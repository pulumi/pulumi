using Pulumi;

class MyStack : Stack
{
    public MyStack()
    {
        // out of bounds exception
        #pragma warning disable
        string[] arr = null;
        #pragma warning disable
        var x = arr[0];
    }
}
