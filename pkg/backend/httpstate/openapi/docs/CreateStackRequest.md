# CreateStackRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**StackName** | **string** | The rest of the StackIdentifier (e.g. organization, project) is in the URL. | 
**Tags** | Pointer to **map[string]string** | An optional set of tags to apply to the stack. | [optional] 

## Methods

### NewCreateStackRequest

`func NewCreateStackRequest(stackName string, ) *CreateStackRequest`

NewCreateStackRequest instantiates a new CreateStackRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewCreateStackRequestWithDefaults

`func NewCreateStackRequestWithDefaults() *CreateStackRequest`

NewCreateStackRequestWithDefaults instantiates a new CreateStackRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetStackName

`func (o *CreateStackRequest) GetStackName() string`

GetStackName returns the StackName field if non-nil, zero value otherwise.

### GetStackNameOk

`func (o *CreateStackRequest) GetStackNameOk() (*string, bool)`

GetStackNameOk returns a tuple with the StackName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStackName

`func (o *CreateStackRequest) SetStackName(v string)`

SetStackName sets StackName field to given value.


### GetTags

`func (o *CreateStackRequest) GetTags() map[string]string`

GetTags returns the Tags field if non-nil, zero value otherwise.

### GetTagsOk

`func (o *CreateStackRequest) GetTagsOk() (*map[string]string, bool)`

GetTagsOk returns a tuple with the Tags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTags

`func (o *CreateStackRequest) SetTags(v map[string]string)`

SetTags sets Tags field to given value.

### HasTags

`func (o *CreateStackRequest) HasTags() bool`

HasTags returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


