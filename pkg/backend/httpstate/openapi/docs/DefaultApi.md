# \DefaultApi

All URIs are relative to *https://api.pulumi.com/api*

Method | HTTP request | Description
------------- | ------------- | -------------
[**GetCLIVersionInfo**](DefaultApi.md#GetCLIVersionInfo) | **Get** /cli/version | getCLIVersionInfo asks the service for information about versions of the CLI (the newest version as well as the oldest version before the CLI should warn about an upgrade).



## GetCLIVersionInfo

> GetCLIVersionInfo200Response GetCLIVersionInfo(ctx).Execute()

getCLIVersionInfo asks the service for information about versions of the CLI (the newest version as well as the oldest version before the CLI should warn about an upgrade).

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
    resp, r, err := apiClient.DefaultApi.GetCLIVersionInfo(context.Background()).Execute()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error when calling `DefaultApi.GetCLIVersionInfo``: %v\n", err)
        fmt.Fprintf(os.Stderr, "Full HTTP response: %v\n", r)
    }
    // response from `GetCLIVersionInfo`: GetCLIVersionInfo200Response
    fmt.Fprintf(os.Stdout, "Response from `DefaultApi.GetCLIVersionInfo`: %v\n", resp)
}
```

### Path Parameters

This endpoint does not need any parameter.

### Other Parameters

Other parameters are passed through a pointer to a apiGetCLIVersionInfoRequest struct via the builder pattern


### Return type

[**GetCLIVersionInfo200Response**](GetCLIVersionInfo200Response.md)

### Authorization

No authorization required

### HTTP request headers

- **Content-Type**: Not defined
- **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints)
[[Back to Model list]](../README.md#documentation-for-models)
[[Back to README]](../README.md)

