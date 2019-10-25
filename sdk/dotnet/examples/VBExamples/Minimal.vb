Imports Pulumi
Imports Pulumi.Azure.Core
Imports Storage = Pulumi.Azure.Storage

Public Class Minimal
    Public Shared Function Run() As IDictionary(Of String, Object)
        Dim resourceGroup = New ResourceGroup("rg", New ResourceGroupArgs With {
            .Location = "West Europe"
        })
        Dim storageAccount = New Storage.Account("sa", New Storage.AccountArgs With {
            .ResourceGroupName = resourceGroup.Name,
            .AccountReplicationType = "LRS",
            .AccountTier = "Standard"
        })
        Return New Dictionary(Of String, Object) From {
            {"accessKey", storageAccount.PrimaryAccessKey}
        }
    End Function
End Class
