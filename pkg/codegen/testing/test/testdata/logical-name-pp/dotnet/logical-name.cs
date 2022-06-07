using Pulumi;
using Random = Pulumi.Random;

class MyStack : Stack
{
    public MyStack()
    {
        var resourceLexicalName = new Random.RandomPet("aA-Alpha_alpha.🤯⁉️", new Random.RandomPetArgs
        {
        });
        this.OutputLexicalName = resourceLexicalName.Id;
    }

    [Output("bB-Beta_beta.💜⁉")]
    public Output<string> OutputLexicalName { get; set; }
}
