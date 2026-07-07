# FieldDefinition

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**Kind** | Pointer to **string** |  | [optional] 
**Href** | Pointer to **string** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**EntityDefinitionId** | **string** |  | 
**FieldName** | **string** |  | 
**FieldType** | **string** |  | 
**IsRequired** | Pointer to **bool** |  | [optional] 

## Methods

### NewFieldDefinition

`func NewFieldDefinition(entityDefinitionId string, fieldName string, fieldType string, ) *FieldDefinition`

NewFieldDefinition instantiates a new FieldDefinition object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewFieldDefinitionWithDefaults

`func NewFieldDefinitionWithDefaults() *FieldDefinition`

NewFieldDefinitionWithDefaults instantiates a new FieldDefinition object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *FieldDefinition) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *FieldDefinition) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *FieldDefinition) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *FieldDefinition) HasId() bool`

HasId returns a boolean if a field has been set.

### GetKind

`func (o *FieldDefinition) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *FieldDefinition) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *FieldDefinition) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *FieldDefinition) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetHref

`func (o *FieldDefinition) GetHref() string`

GetHref returns the Href field if non-nil, zero value otherwise.

### GetHrefOk

`func (o *FieldDefinition) GetHrefOk() (*string, bool)`

GetHrefOk returns a tuple with the Href field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHref

`func (o *FieldDefinition) SetHref(v string)`

SetHref sets Href field to given value.

### HasHref

`func (o *FieldDefinition) HasHref() bool`

HasHref returns a boolean if a field has been set.

### GetCreatedAt

`func (o *FieldDefinition) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *FieldDefinition) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *FieldDefinition) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *FieldDefinition) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *FieldDefinition) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *FieldDefinition) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *FieldDefinition) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *FieldDefinition) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetEntityDefinitionId

`func (o *FieldDefinition) GetEntityDefinitionId() string`

GetEntityDefinitionId returns the EntityDefinitionId field if non-nil, zero value otherwise.

### GetEntityDefinitionIdOk

`func (o *FieldDefinition) GetEntityDefinitionIdOk() (*string, bool)`

GetEntityDefinitionIdOk returns a tuple with the EntityDefinitionId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEntityDefinitionId

`func (o *FieldDefinition) SetEntityDefinitionId(v string)`

SetEntityDefinitionId sets EntityDefinitionId field to given value.


### GetFieldName

`func (o *FieldDefinition) GetFieldName() string`

GetFieldName returns the FieldName field if non-nil, zero value otherwise.

### GetFieldNameOk

`func (o *FieldDefinition) GetFieldNameOk() (*string, bool)`

GetFieldNameOk returns a tuple with the FieldName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFieldName

`func (o *FieldDefinition) SetFieldName(v string)`

SetFieldName sets FieldName field to given value.


### GetFieldType

`func (o *FieldDefinition) GetFieldType() string`

GetFieldType returns the FieldType field if non-nil, zero value otherwise.

### GetFieldTypeOk

`func (o *FieldDefinition) GetFieldTypeOk() (*string, bool)`

GetFieldTypeOk returns a tuple with the FieldType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFieldType

`func (o *FieldDefinition) SetFieldType(v string)`

SetFieldType sets FieldType field to given value.


### GetIsRequired

`func (o *FieldDefinition) GetIsRequired() bool`

GetIsRequired returns the IsRequired field if non-nil, zero value otherwise.

### GetIsRequiredOk

`func (o *FieldDefinition) GetIsRequiredOk() (*bool, bool)`

GetIsRequiredOk returns a tuple with the IsRequired field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetIsRequired

`func (o *FieldDefinition) SetIsRequired(v bool)`

SetIsRequired sets IsRequired field to given value.

### HasIsRequired

`func (o *FieldDefinition) HasIsRequired() bool`

HasIsRequired returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


