# Relationship

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**Kind** | Pointer to **string** |  | [optional] 
**Href** | Pointer to **string** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**ProjectId** | **string** |  | 
**SourceEntityId** | **string** |  | 
**TargetEntityId** | **string** |  | 
**RelationshipType** | **string** |  | 
**ForeignKey** | Pointer to **string** |  | [optional] 

## Methods

### NewRelationship

`func NewRelationship(projectId string, sourceEntityId string, targetEntityId string, relationshipType string, ) *Relationship`

NewRelationship instantiates a new Relationship object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewRelationshipWithDefaults

`func NewRelationshipWithDefaults() *Relationship`

NewRelationshipWithDefaults instantiates a new Relationship object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *Relationship) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *Relationship) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *Relationship) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *Relationship) HasId() bool`

HasId returns a boolean if a field has been set.

### GetKind

`func (o *Relationship) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *Relationship) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *Relationship) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *Relationship) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetHref

`func (o *Relationship) GetHref() string`

GetHref returns the Href field if non-nil, zero value otherwise.

### GetHrefOk

`func (o *Relationship) GetHrefOk() (*string, bool)`

GetHrefOk returns a tuple with the Href field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHref

`func (o *Relationship) SetHref(v string)`

SetHref sets Href field to given value.

### HasHref

`func (o *Relationship) HasHref() bool`

HasHref returns a boolean if a field has been set.

### GetCreatedAt

`func (o *Relationship) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *Relationship) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *Relationship) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *Relationship) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *Relationship) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *Relationship) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *Relationship) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *Relationship) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetProjectId

`func (o *Relationship) GetProjectId() string`

GetProjectId returns the ProjectId field if non-nil, zero value otherwise.

### GetProjectIdOk

`func (o *Relationship) GetProjectIdOk() (*string, bool)`

GetProjectIdOk returns a tuple with the ProjectId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProjectId

`func (o *Relationship) SetProjectId(v string)`

SetProjectId sets ProjectId field to given value.


### GetSourceEntityId

`func (o *Relationship) GetSourceEntityId() string`

GetSourceEntityId returns the SourceEntityId field if non-nil, zero value otherwise.

### GetSourceEntityIdOk

`func (o *Relationship) GetSourceEntityIdOk() (*string, bool)`

GetSourceEntityIdOk returns a tuple with the SourceEntityId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSourceEntityId

`func (o *Relationship) SetSourceEntityId(v string)`

SetSourceEntityId sets SourceEntityId field to given value.


### GetTargetEntityId

`func (o *Relationship) GetTargetEntityId() string`

GetTargetEntityId returns the TargetEntityId field if non-nil, zero value otherwise.

### GetTargetEntityIdOk

`func (o *Relationship) GetTargetEntityIdOk() (*string, bool)`

GetTargetEntityIdOk returns a tuple with the TargetEntityId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTargetEntityId

`func (o *Relationship) SetTargetEntityId(v string)`

SetTargetEntityId sets TargetEntityId field to given value.


### GetRelationshipType

`func (o *Relationship) GetRelationshipType() string`

GetRelationshipType returns the RelationshipType field if non-nil, zero value otherwise.

### GetRelationshipTypeOk

`func (o *Relationship) GetRelationshipTypeOk() (*string, bool)`

GetRelationshipTypeOk returns a tuple with the RelationshipType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetRelationshipType

`func (o *Relationship) SetRelationshipType(v string)`

SetRelationshipType sets RelationshipType field to given value.


### GetForeignKey

`func (o *Relationship) GetForeignKey() string`

GetForeignKey returns the ForeignKey field if non-nil, zero value otherwise.

### GetForeignKeyOk

`func (o *Relationship) GetForeignKeyOk() (*string, bool)`

GetForeignKeyOk returns a tuple with the ForeignKey field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetForeignKey

`func (o *Relationship) SetForeignKey(v string)`

SetForeignKey sets ForeignKey field to given value.

### HasForeignKey

`func (o *Relationship) HasForeignKey() bool`

HasForeignKey returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


