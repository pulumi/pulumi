# ListStacks200Response

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Stacks** | Pointer to [**[]StackSummary**](StackSummary.md) |  | [optional] 
**ContinuationToken** | Pointer to **string** | ContinuationToken is an opaque value used to mark the end of the all stacks. If non-nil, pass it into a subsequent call in order to get the next batch of results. A value of nil means that all stacks have been returned.  | [optional] 

## Methods

### NewListStacks200Response

`func NewListStacks200Response() *ListStacks200Response`

NewListStacks200Response instantiates a new ListStacks200Response object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewListStacks200ResponseWithDefaults

`func NewListStacks200ResponseWithDefaults() *ListStacks200Response`

NewListStacks200ResponseWithDefaults instantiates a new ListStacks200Response object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetStacks

`func (o *ListStacks200Response) GetStacks() []StackSummary`

GetStacks returns the Stacks field if non-nil, zero value otherwise.

### GetStacksOk

`func (o *ListStacks200Response) GetStacksOk() (*[]StackSummary, bool)`

GetStacksOk returns a tuple with the Stacks field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStacks

`func (o *ListStacks200Response) SetStacks(v []StackSummary)`

SetStacks sets Stacks field to given value.

### HasStacks

`func (o *ListStacks200Response) HasStacks() bool`

HasStacks returns a boolean if a field has been set.

### GetContinuationToken

`func (o *ListStacks200Response) GetContinuationToken() string`

GetContinuationToken returns the ContinuationToken field if non-nil, zero value otherwise.

### GetContinuationTokenOk

`func (o *ListStacks200Response) GetContinuationTokenOk() (*string, bool)`

GetContinuationTokenOk returns a tuple with the ContinuationToken field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetContinuationToken

`func (o *ListStacks200Response) SetContinuationToken(v string)`

SetContinuationToken sets ContinuationToken field to given value.

### HasContinuationToken

`func (o *ListStacks200Response) HasContinuationToken() bool`

HasContinuationToken returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


