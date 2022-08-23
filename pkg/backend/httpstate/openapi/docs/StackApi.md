# \StackApi

All URIs are relative to *https://api.pulumi.com/api*

Method | HTTP request | Description
------------- | ------------- | -------------
[**CreateStack**](StackApi.md#CreateStack) | **Post** /{organization}/{project} | CreateStack creates a stack with the given cloud and stack name in the scope of the indicated project.
[**DecryptValue**](StackApi.md#DecryptValue) | **Post** /{organization}/{project}/{stack}/decrypt | DecryptValue decrypts a ciphertext value in the context of the indicated stack.
[**DeleteStack**](StackApi.md#DeleteStack) | **Delete** /{organization}/{project}/{stack} | DeleteStack deletes the indicated stack. If force is true, the stack is deleted even if it contains resources.
[**DoesProjectExist**](StackApi.md#DoesProjectExist) | **Head** /stacks/{organization}/{project} | Returns true if a project with the given name exists, or false otherwise.
[**EncryptValue**](StackApi.md#EncryptValue) | **Post** /{organization}/{project}/{stack}/encrypt | EncryptValue encrypts a plaintext value in the context of the indicated stack.
[**GetStack**](StackApi.md#GetStack) | **Get** /{organization}/{project}/{stack} | GetStack retrieves the stack with the given name.
[**UpdateStackTags**](StackApi.md#UpdateStackTags) | **Patch** /{organization}/{project}/{stack}/tags | UpdateStackTags updates the stacks&#39;s tags, replacing all existing tags.



## CreateStack

> Stack CreateStack(ctx, organization, project).CreateStackRequest(createStackRequest).Execute()

CreateStack creates a stack with the given cloud and stack name in the scope of the indicated project.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 
    createStackRequest := *openapiclient.NewCreateStackRequest("StackName_example") // CreateStackRequest | CreateStackRequest defines the request body for creating a new Stack

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.CreateStack(context.Background(), organization, project).CreateStackRequest(createStackRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.CreateStack``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `CreateStack`: Stack
    fmt.Fprintf(os.Stdout, "Response from `StackApi.CreateStack`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiCreateStackRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------


 **createStackRequest** | [**CreateStackRequest**](CreateStackRequest.md) | CreateStackRequest defines the request body for creating a new Stack | 

### Return type

[**Stack**](Stack.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DecryptValue

> DecryptValueResponse DecryptValue(ctx, organization, project, stack).DecryptValueRequest(decryptValueRequest).Execute()

DecryptValue decrypts a ciphertext value in the context of the indicated stack.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 
    stack := "stack_example" // string | 
    decryptValueRequest := *openapiclient.NewDecryptValueRequest(string(123)) // DecryptValueRequest | 

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.DecryptValue(context.Background(), organization, project, stack).DecryptValueRequest(decryptValueRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.DecryptValue``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `DecryptValue`: DecryptValueResponse
    fmt.Fprintf(os.Stdout, "Response from `StackApi.DecryptValue`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 
**stack** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiDecryptValueRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **decryptValueRequest** | [**DecryptValueRequest**](DecryptValueRequest.md) |  | 

### Return type

[**DecryptValueResponse**](DecryptValueResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DeleteStack

> map[string]interface{} DeleteStack(ctx, organization, project, stack).Force(force).Execute()

DeleteStack deletes the indicated stack. If force is true, the stack is deleted even if it contains resources.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 
    stack := "stack_example" // string | 
    force := true // bool |  (optional) (default to false)

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.DeleteStack(context.Background(), organization, project, stack).Force(force).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.DeleteStack``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `DeleteStack`: map[string]interface{}
    fmt.Fprintf(os.Stdout, "Response from `StackApi.DeleteStack`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 
**stack** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiDeleteStackRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **force** | **bool** |  | [default to false]

### Return type

**map[string]interface{}**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## DoesProjectExist

> map[string]interface{} DoesProjectExist(ctx, organization, project).Execute()

Returns true if a project with the given name exists, or false otherwise.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.DoesProjectExist(context.Background(), organization, project).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.DoesProjectExist``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `DoesProjectExist`: map[string]interface{}
    fmt.Fprintf(os.Stdout, "Response from `StackApi.DoesProjectExist`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiDoesProjectExistRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



### Return type

**map[string]interface{}**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## EncryptValue

> EncryptValueResponse EncryptValue(ctx, organization, project, stack).EncryptValueRequest(encryptValueRequest).Execute()

EncryptValue encrypts a plaintext value in the context of the indicated stack.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 
    stack := "stack_example" // string | 
    encryptValueRequest := *openapiclient.NewEncryptValueRequest(string(123)) // EncryptValueRequest | 

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.EncryptValue(context.Background(), organization, project, stack).EncryptValueRequest(encryptValueRequest).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.EncryptValue``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `EncryptValue`: EncryptValueResponse
    fmt.Fprintf(os.Stdout, "Response from `StackApi.EncryptValue`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 
**stack** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiEncryptValueRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **encryptValueRequest** | [**EncryptValueRequest**](EncryptValueRequest.md) |  | 

### Return type

[**EncryptValueResponse**](EncryptValueResponse.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## GetStack

> Stack GetStack(ctx, organization, project, stack).Execute()

GetStack retrieves the stack with the given name.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 
    stack := "stack_example" // string | 

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.GetStack(context.Background(), organization, project, stack).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.GetStack``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetStack`: Stack
    fmt.Fprintf(os.Stdout, "Response from `StackApi.GetStack`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 
**stack** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiGetStackRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------




### Return type

[**Stack**](Stack.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## UpdateStackTags

> map[string]interface{} UpdateStackTags(ctx, organization, project, stack).RequestBody(requestBody).Execute()

UpdateStackTags updates the stacks's tags, replacing all existing tags.

### Example

```go
package main

import (
    "context"
    "fmt"
    "os"
    openapiclient "./openapi"
)

func main() {
    organization := "organization_example" // string | 
    project := "project_example" // string | 
    stack := "stack_example" // string | 
    requestBody := map[string]string{"key": "Inner_example"} // map[string]string | 

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.StackApi.UpdateStackTags(context.Background(), organization, project, stack).RequestBody(requestBody).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `StackApi.UpdateStackTags``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `UpdateStackTags`: map[string]interface{}
    fmt.Fprintf(os.Stdout, "Response from `StackApi.UpdateStackTags`: %v\n", resp)
}
```

### Path Parameters


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
**ctx** | **context.Context** | context for authentication, logging, cancellation, deadlines, tracing, etc.
**organization** | **string** |  | 
**project** | **string** |  | 
**stack** | **string** |  | 

### Other Parameters

Other parameters are passed through a pointer to a apiUpdateStackTagsRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------



 **requestBody** | **map[string]string** |  | 

### Return type

**map[string]interface{}**

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: application/json
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

