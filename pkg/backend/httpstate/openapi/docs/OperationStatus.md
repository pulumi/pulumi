# OperationStatus

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Kind** | Pointer to [**UpdateKind**](UpdateKind.md) |  | [optional] 
**Author** | Pointer to **string** |  | [optional] 
**Started** | Pointer to **int64** |  | [optional] 

## Methods

### NewOperationStatus

`func NewOperationStatus() *OperationStatus`

NewOperationStatus instantiates a new OperationStatus object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewOperationStatusWithDefaults

`func NewOperationStatusWithDefaults() *OperationStatus`

NewOperationStatusWithDefaults instantiates a new OperationStatus object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetKind

`func (o *OperationStatus) GetKind() UpdateKind`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *OperationStatus) GetKindOk() (*UpdateKind, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *OperationStatus) SetKind(v UpdateKind)`

SetKind sets Kind field to given value.

### HasKind

`func (o *OperationStatus) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetAuthor

`func (o *OperationStatus) GetAuthor() string`

GetAuthor returns the Author field if non-nil, zero value otherwise.

### GetAuthorOk

`func (o *OperationStatus) GetAuthorOk() (*string, bool)`

GetAuthorOk returns a tuple with the Author field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAuthor

`func (o *OperationStatus) SetAuthor(v string)`

SetAuthor sets Author field to given value.

### HasAuthor

`func (o *OperationStatus) HasAuthor() bool`

HasAuthor returns a boolean if a field has been set.

### GetStarted

`func (o *OperationStatus) GetStarted() int64`

GetStarted returns the Started field if non-nil, zero value otherwise.

### GetStartedOk

`func (o *OperationStatus) GetStartedOk() (*int64, bool)`

GetStartedOk returns a tuple with the Started field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStarted

`func (o *OperationStatus) SetStarted(v int64)`

SetStarted sets Started field to given value.

### HasStarted

`func (o *OperationStatus) HasStarted() bool`

HasStarted returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


