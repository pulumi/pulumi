# Stack

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**OrgName** | Pointer to **string** |  | [optional] 
**ProjectName** | Pointer to **string** |  | [optional] 
**StackName** | Pointer to **string** |  | [optional] 
**ActiveUpdate** | Pointer to **string** |  | [optional] 
**Tags** | Pointer to **map[string]string** | Tags are a set of key values applied to stacks. | [optional] 
**Version** | Pointer to **int32** |  | [optional] 
**CurrentOperation** | Pointer to [**OperationStatus**](OperationStatus.md) |  | [optional] 

## Methods

### NewStack

`func NewStack() *Stack`

NewStack instantiates a new Stack object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewStackWithDefaults

`func NewStackWithDefaults() *Stack`

NewStackWithDefaults instantiates a new Stack object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetOrgName

`func (o *Stack) GetOrgName() string`

GetOrgName returns the OrgName field if non-nil, zero value otherwise.

### GetOrgNameOk

`func (o *Stack) GetOrgNameOk() (*string, bool)`

GetOrgNameOk returns a tuple with the OrgName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOrgName

`func (o *Stack) SetOrgName(v string)`

SetOrgName sets OrgName field to given value.

### HasOrgName

`func (o *Stack) HasOrgName() bool`

HasOrgName returns a boolean if a field has been set.

### GetProjectName

`func (o *Stack) GetProjectName() string`

GetProjectName returns the ProjectName field if non-nil, zero value otherwise.

### GetProjectNameOk

`func (o *Stack) GetProjectNameOk() (*string, bool)`

GetProjectNameOk returns a tuple with the ProjectName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProjectName

`func (o *Stack) SetProjectName(v string)`

SetProjectName sets ProjectName field to given value.

### HasProjectName

`func (o *Stack) HasProjectName() bool`

HasProjectName returns a boolean if a field has been set.

### GetStackName

`func (o *Stack) GetStackName() string`

GetStackName returns the StackName field if non-nil, zero value otherwise.

### GetStackNameOk

`func (o *Stack) GetStackNameOk() (*string, bool)`

GetStackNameOk returns a tuple with the StackName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStackName

`func (o *Stack) SetStackName(v string)`

SetStackName sets StackName field to given value.

### HasStackName

`func (o *Stack) HasStackName() bool`

HasStackName returns a boolean if a field has been set.

### GetActiveUpdate

`func (o *Stack) GetActiveUpdate() string`

GetActiveUpdate returns the ActiveUpdate field if non-nil, zero value otherwise.

### GetActiveUpdateOk

`func (o *Stack) GetActiveUpdateOk() (*string, bool)`

GetActiveUpdateOk returns a tuple with the ActiveUpdate field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetActiveUpdate

`func (o *Stack) SetActiveUpdate(v string)`

SetActiveUpdate sets ActiveUpdate field to given value.

### HasActiveUpdate

`func (o *Stack) HasActiveUpdate() bool`

HasActiveUpdate returns a boolean if a field has been set.

### GetTags

`func (o *Stack) GetTags() map[string]string`

GetTags returns the Tags field if non-nil, zero value otherwise.

### GetTagsOk

`func (o *Stack) GetTagsOk() (*map[string]string, bool)`

GetTagsOk returns a tuple with the Tags field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTags

`func (o *Stack) SetTags(v map[string]string)`

SetTags sets Tags field to given value.

### HasTags

`func (o *Stack) HasTags() bool`

HasTags returns a boolean if a field has been set.

### GetVersion

`func (o *Stack) GetVersion() int32`

GetVersion returns the Version field if non-nil, zero value otherwise.

### GetVersionOk

`func (o *Stack) GetVersionOk() (*int32, bool)`

GetVersionOk returns a tuple with the Version field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetVersion

`func (o *Stack) SetVersion(v int32)`

SetVersion sets Version field to given value.

### HasVersion

`func (o *Stack) HasVersion() bool`

HasVersion returns a boolean if a field has been set.

### GetCurrentOperation

`func (o *Stack) GetCurrentOperation() OperationStatus`

GetCurrentOperation returns the CurrentOperation field if non-nil, zero value otherwise.

### GetCurrentOperationOk

`func (o *Stack) GetCurrentOperationOk() (*OperationStatus, bool)`

GetCurrentOperationOk returns a tuple with the CurrentOperation field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCurrentOperation

`func (o *Stack) SetCurrentOperation(v OperationStatus)`

SetCurrentOperation sets CurrentOperation field to given value.

### HasCurrentOperation

`func (o *Stack) HasCurrentOperation() bool`

HasCurrentOperation returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


