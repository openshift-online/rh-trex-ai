# EntityDefinition

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**Kind** | Pointer to **string** |  | [optional] 
**Href** | Pointer to **string** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**ProjectId** | **string** |  | 
**KindName** | **string** |  | 
**PluralOverride** | Pointer to **string** |  | [optional] 
**Description** | Pointer to **string** |  | [optional] 

## Methods

### NewEntityDefinition

`func NewEntityDefinition(projectId string, kindName string, ) *EntityDefinition`

NewEntityDefinition instantiates a new EntityDefinition object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewEntityDefinitionWithDefaults

`func NewEntityDefinitionWithDefaults() *EntityDefinition`

NewEntityDefinitionWithDefaults instantiates a new EntityDefinition object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *EntityDefinition) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *EntityDefinition) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *EntityDefinition) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *EntityDefinition) HasId() bool`

HasId returns a boolean if a field has been set.

### GetKind

`func (o *EntityDefinition) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *EntityDefinition) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *EntityDefinition) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *EntityDefinition) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetHref

`func (o *EntityDefinition) GetHref() string`

GetHref returns the Href field if non-nil, zero value otherwise.

### GetHrefOk

`func (o *EntityDefinition) GetHrefOk() (*string, bool)`

GetHrefOk returns a tuple with the Href field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHref

`func (o *EntityDefinition) SetHref(v string)`

SetHref sets Href field to given value.

### HasHref

`func (o *EntityDefinition) HasHref() bool`

HasHref returns a boolean if a field has been set.

### GetCreatedAt

`func (o *EntityDefinition) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *EntityDefinition) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *EntityDefinition) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *EntityDefinition) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *EntityDefinition) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *EntityDefinition) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *EntityDefinition) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *EntityDefinition) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetProjectId

`func (o *EntityDefinition) GetProjectId() string`

GetProjectId returns the ProjectId field if non-nil, zero value otherwise.

### GetProjectIdOk

`func (o *EntityDefinition) GetProjectIdOk() (*string, bool)`

GetProjectIdOk returns a tuple with the ProjectId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProjectId

`func (o *EntityDefinition) SetProjectId(v string)`

SetProjectId sets ProjectId field to given value.


### GetKindName

`func (o *EntityDefinition) GetKindName() string`

GetKindName returns the KindName field if non-nil, zero value otherwise.

### GetKindNameOk

`func (o *EntityDefinition) GetKindNameOk() (*string, bool)`

GetKindNameOk returns a tuple with the KindName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKindName

`func (o *EntityDefinition) SetKindName(v string)`

SetKindName sets KindName field to given value.


### GetPluralOverride

`func (o *EntityDefinition) GetPluralOverride() string`

GetPluralOverride returns the PluralOverride field if non-nil, zero value otherwise.

### GetPluralOverrideOk

`func (o *EntityDefinition) GetPluralOverrideOk() (*string, bool)`

GetPluralOverrideOk returns a tuple with the PluralOverride field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPluralOverride

`func (o *EntityDefinition) SetPluralOverride(v string)`

SetPluralOverride sets PluralOverride field to given value.

### HasPluralOverride

`func (o *EntityDefinition) HasPluralOverride() bool`

HasPluralOverride returns a boolean if a field has been set.

### GetDescription

`func (o *EntityDefinition) GetDescription() string`

GetDescription returns the Description field if non-nil, zero value otherwise.

### GetDescriptionOk

`func (o *EntityDefinition) GetDescriptionOk() (*string, bool)`

GetDescriptionOk returns a tuple with the Description field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDescription

`func (o *EntityDefinition) SetDescription(v string)`

SetDescription sets Description field to given value.

### HasDescription

`func (o *EntityDefinition) HasDescription() bool`

HasDescription returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


