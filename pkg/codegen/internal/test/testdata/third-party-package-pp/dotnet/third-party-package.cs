using Pulumi;
using Other = ThirdParty.Other;

class MyStack : Stack
{
    public MyStack()
    {
        var Other = new Other.Thing("Other", new Other.ThingArgs
        {
            Idea = "Support Third Party",
        });
        var Question = new Other.Module.Object("Question", new Other.Module.ObjectArgs
        {
            Answer = 42,
        });
    }

}
