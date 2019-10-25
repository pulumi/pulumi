Imports Pulumi

Module Program

    Sub Main()
        Deployment.RunAsync(AddressOf Minimal.Run).Wait()
    End Sub

End Module