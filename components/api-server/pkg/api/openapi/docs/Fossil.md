# Fossil

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Id** | Pointer to **string** |  | [optional] 
**Kind** | Pointer to **string** |  | [optional] 
**Href** | Pointer to **string** |  | [optional] 
**CreatedAt** | Pointer to **time.Time** |  | [optional] 
**UpdatedAt** | Pointer to **time.Time** |  | [optional] 
**DiscoveryLocation** | **string** |  | 
**EstimatedAge** | Pointer to **int32** |  | [optional] 
**FossilType** | Pointer to **string** |  | [optional] 
**ExcavatorName** | Pointer to **string** |  | [optional] 

## Methods

### NewFossil

`func NewFossil(discoveryLocation string, ) *Fossil`

NewFossil instantiates a new Fossil object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewFossilWithDefaults

`func NewFossilWithDefaults() *Fossil`

NewFossilWithDefaults instantiates a new Fossil object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetId

`func (o *Fossil) GetId() string`

GetId returns the Id field if non-nil, zero value otherwise.

### GetIdOk

`func (o *Fossil) GetIdOk() (*string, bool)`

GetIdOk returns a tuple with the Id field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetId

`func (o *Fossil) SetId(v string)`

SetId sets Id field to given value.

### HasId

`func (o *Fossil) HasId() bool`

HasId returns a boolean if a field has been set.

### GetKind

`func (o *Fossil) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *Fossil) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *Fossil) SetKind(v string)`

SetKind sets Kind field to given value.

### HasKind

`func (o *Fossil) HasKind() bool`

HasKind returns a boolean if a field has been set.

### GetHref

`func (o *Fossil) GetHref() string`

GetHref returns the Href field if non-nil, zero value otherwise.

### GetHrefOk

`func (o *Fossil) GetHrefOk() (*string, bool)`

GetHrefOk returns a tuple with the Href field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetHref

`func (o *Fossil) SetHref(v string)`

SetHref sets Href field to given value.

### HasHref

`func (o *Fossil) HasHref() bool`

HasHref returns a boolean if a field has been set.

### GetCreatedAt

`func (o *Fossil) GetCreatedAt() time.Time`

GetCreatedAt returns the CreatedAt field if non-nil, zero value otherwise.

### GetCreatedAtOk

`func (o *Fossil) GetCreatedAtOk() (*time.Time, bool)`

GetCreatedAtOk returns a tuple with the CreatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCreatedAt

`func (o *Fossil) SetCreatedAt(v time.Time)`

SetCreatedAt sets CreatedAt field to given value.

### HasCreatedAt

`func (o *Fossil) HasCreatedAt() bool`

HasCreatedAt returns a boolean if a field has been set.

### GetUpdatedAt

`func (o *Fossil) GetUpdatedAt() time.Time`

GetUpdatedAt returns the UpdatedAt field if non-nil, zero value otherwise.

### GetUpdatedAtOk

`func (o *Fossil) GetUpdatedAtOk() (*time.Time, bool)`

GetUpdatedAtOk returns a tuple with the UpdatedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetUpdatedAt

`func (o *Fossil) SetUpdatedAt(v time.Time)`

SetUpdatedAt sets UpdatedAt field to given value.

### HasUpdatedAt

`func (o *Fossil) HasUpdatedAt() bool`

HasUpdatedAt returns a boolean if a field has been set.

### GetDiscoveryLocation

`func (o *Fossil) GetDiscoveryLocation() string`

GetDiscoveryLocation returns the DiscoveryLocation field if non-nil, zero value otherwise.

### GetDiscoveryLocationOk

`func (o *Fossil) GetDiscoveryLocationOk() (*string, bool)`

GetDiscoveryLocationOk returns a tuple with the DiscoveryLocation field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetDiscoveryLocation

`func (o *Fossil) SetDiscoveryLocation(v string)`

SetDiscoveryLocation sets DiscoveryLocation field to given value.


### GetEstimatedAge

`func (o *Fossil) GetEstimatedAge() int32`

GetEstimatedAge returns the EstimatedAge field if non-nil, zero value otherwise.

### GetEstimatedAgeOk

`func (o *Fossil) GetEstimatedAgeOk() (*int32, bool)`

GetEstimatedAgeOk returns a tuple with the EstimatedAge field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetEstimatedAge

`func (o *Fossil) SetEstimatedAge(v int32)`

SetEstimatedAge sets EstimatedAge field to given value.

### HasEstimatedAge

`func (o *Fossil) HasEstimatedAge() bool`

HasEstimatedAge returns a boolean if a field has been set.

### GetFossilType

`func (o *Fossil) GetFossilType() string`

GetFossilType returns the FossilType field if non-nil, zero value otherwise.

### GetFossilTypeOk

`func (o *Fossil) GetFossilTypeOk() (*string, bool)`

GetFossilTypeOk returns a tuple with the FossilType field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetFossilType

`func (o *Fossil) SetFossilType(v string)`

SetFossilType sets FossilType field to given value.

### HasFossilType

`func (o *Fossil) HasFossilType() bool`

HasFossilType returns a boolean if a field has been set.

### GetExcavatorName

`func (o *Fossil) GetExcavatorName() string`

GetExcavatorName returns the ExcavatorName field if non-nil, zero value otherwise.

### GetExcavatorNameOk

`func (o *Fossil) GetExcavatorNameOk() (*string, bool)`

GetExcavatorNameOk returns a tuple with the ExcavatorName field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetExcavatorName

`func (o *Fossil) SetExcavatorName(v string)`

SetExcavatorName sets ExcavatorName field to given value.

### HasExcavatorName

`func (o *Fossil) HasExcavatorName() bool`

HasExcavatorName returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


