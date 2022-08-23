# \UserApi

All URIs are relative to *https://api.pulumi.com/api*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetCurrentUser**](UserApi.md#GetCurrentUser) | **Get** /user | Returns the current user.
[**ListStacks**](UserApi.md#ListStacks) | **Get** /user/stacks | Lists all stacks the current user has access to, optionally filtered by project.



## GetCurrentUser

> ServiceUser GetCurrentUser(ctx).Execute()

Returns the current user.

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

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.UserApi.GetCurrentUser(context.Background()).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `UserApi.GetCurrentUser``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetCurrentUser`: ServiceUser
    fmt.Fprintf(os.Stdout, "Response from `UserApi.GetCurrentUser`: %v\n", resp)
}
```

### Path Parameters

This endpoint does not need any parameter.

### Other Parameters

Other parameters are passed through a pointer to a apiGetCurrentUserRequest struct via the builder pattern


### Return type

[**ServiceUser**](ServiceUser.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)


## ListStacks

> ListStacks200Response ListStacks(ctx).Project(project).Organization(organization).TagName(tagName).TagValue(tagValue).ContinuationToken(continuationToken).Execute()

Lists all stacks the current user has access to, optionally filtered by project.

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
    project := "project_example" // string |  (optional)
    organization := "organization_example" // string |  (optional)
    tagName := "tagName_example" // string |  (optional)
    tagValue := "tagValue_example" // string |  (optional)
    continuationToken := "continuationToken_example" // string |  (optional)

    configuration := openapiclient.NewConfiguration()
    apiClient := openapiclient.NewAPIClient(configuration)
    resp, r, err := apiClient.UserApi.ListStacks(context.Background()).Project(project).Organization(organization).TagName(tagName).TagValue(tagValue).ContinuationToken(continuationToken).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `UserApi.ListStacks``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `ListStacks`: ListStacks200Response
    fmt.Fprintf(os.Stdout, "Response from `UserApi.ListStacks`: %v\n", resp)
}
```

### Path Parameters



### Other Parameters

Other parameters are passed through a pointer to a apiListStacksRequest struct via the builder pattern


Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **project** | **string** |  | 
 **organization** | **string** |  | 
 **tagName** | **string** |  | 
 **tagValue** | **string** |  | 
 **continuationToken** | **string** |  | 

### Return type

[**ListStacks200Response**](ListStacks200Response.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

