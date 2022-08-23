# ServiceUser

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**GithubLogin** | Pointer to **string** |  | [optional] 
**Name** | Pointer to **string** |  | [optional] 
**Email** | Pointer to **string** |  | [optional] 
**AvatarUrl** | Pointer to **string** |  | [optional] 
**Organizations** | Pointer to [**[]ServiceUserInfo**](ServiceUserInfo.md) |  | [optional] 
**Identities** | Pointer to **[]string** |  | [optional] 
**SiteAdmin** | Pointer to **bool** |  | [optional] [default to false]

## Methods

### NewServiceUser

`func NewServiceUser() *ServiceUser`

NewServiceUser instantiates a new ServiceUser object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewServiceUserWithDefaults

`func NewServiceUserWithDefaults() *ServiceUser`

NewServiceUserWithDefaults instantiates a new ServiceUser object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *ServiceUser) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *ServiceUser) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *ServiceUser) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *ServiceUser) HasId() bool`

HasId returns a boolean if a field has been set.

### GetGithubLogin

`func (o *ServiceUser) GetGithubLogin() string`

GetGithubLogin returns the GithubLogin field if non-nil, zero value otherwise.

### GetGithubLoginOk

`func (o *ServiceUser) GetGithubLoginOk() (*string, bool)`

GetGithubLoginOk returns a tuple with the GithubLogin field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetGithubLogin

`func (o *ServiceUser) SetGithubLogin(v string)`

SetGithubLogin sets GithubLogin field to given value.

### HasGithubLogin

`func (o *ServiceUser) HasGithubLogin() bool`

HasGithubLogin returns a boolean if a field has been set.

### GetName

`func (o *ServiceUser) GetName() string`

GetName returns the Name field if non-nil, zero value otherwise.

### GetNameOk

`func (o *ServiceUser) GetNameOk() (*string, bool)`

GetNameOk returns a tuple with the Name field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetName

`func (o *ServiceUser) SetName(v string)`

SetName sets Name field to given value.

### HasName

`func (o *ServiceUser) HasName() bool`

HasName returns a boolean if a field has been set.

### GetEmail

`func (o *ServiceUser) GetEmail() string`

GetEmail returns the Email field if non-nil, zero value otherwise.

### GetEmailOk

`func (o *ServiceUser) GetEmailOk() (*string, bool)`

GetEmailOk returns a tuple with the Email field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEmail

`func (o *ServiceUser) SetEmail(v string)`

SetEmail sets Email field to given value.

### HasEmail

`func (o *ServiceUser) HasEmail() bool`

HasEmail returns a boolean if a field has been set.

### GetAvatarUrl

`func (o *ServiceUser) GetAvatarUrl() string`

GetAvatarUrl returns the AvatarUrl field if non-nil, zero value otherwise.

### GetAvatarUrlOk

`func (o *ServiceUser) GetAvatarUrlOk() (*string, bool)`

GetAvatarUrlOk returns a tuple with the AvatarUrl field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetAvatarUrl

`func (o *ServiceUser) SetAvatarUrl(v string)`

SetAvatarUrl sets AvatarUrl field to given value.

### HasAvatarUrl

`func (o *ServiceUser) HasAvatarUrl() bool`

HasAvatarUrl returns a boolean if a field has been set.

### GetOrganizations

`func (o *ServiceUser) GetOrganizations() []ServiceUserInfo`

GetOrganizations returns the Organizations field if non-nil, zero value otherwise.

### GetOrganizationsOk

`func (o *ServiceUser) GetOrganizationsOk() (*[]ServiceUserInfo, bool)`

GetOrganizationsOk returns a tuple with the Organizations field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetOrganizations

`func (o *ServiceUser) SetOrganizations(v []ServiceUserInfo)`

SetOrganizations sets Organizations field to given value.

### HasOrganizations

`func (o *ServiceUser) HasOrganizations() bool`

HasOrganizations returns a boolean if a field has been set.

### GetIdentities

`func (o *ServiceUser) GetIdentities() []string`

GetIdentities returns the Identities field if non-nil, zero value otherwise.

### GetIdentitiesOk

`func (o *ServiceUser) GetIdentitiesOk() (*[]string, bool)`

GetIdentitiesOk returns a tuple with the Identities field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIdentities

`func (o *ServiceUser) SetIdentities(v []string)`

SetIdentities sets Identities field to given value.

### HasIdentities

`func (o *ServiceUser) HasIdentities() bool`

HasIdentities returns a boolean if a field has been set.

### GetSiteAdmin

`func (o *ServiceUser) GetSiteAdmin() bool`

GetSiteAdmin returns the SiteAdmin field if non-nil, zero value otherwise.

### GetSiteAdminOk

`func (o *ServiceUser) GetSiteAdminOk() (*bool, bool)`

GetSiteAdminOk returns a tuple with the SiteAdmin field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSiteAdmin

`func (o *ServiceUser) SetSiteAdmin(v bool)`

SetSiteAdmin sets SiteAdmin field to given value.

### HasSiteAdmin

`func (o *ServiceUser) HasSiteAdmin() bool`

HasSiteAdmin returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


