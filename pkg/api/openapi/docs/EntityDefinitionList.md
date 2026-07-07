# EntityDefinitionList

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Kind** | **string** |  | 
**Page** | **int32** |  | 
**Size** | **int32** |  | 
**Total** | **int32** |  | 
**Items** | [**[]EntityDefinition**](EntityDefinition.md) |  | 

## Methods

### NewEntityDefinitionList

`func NewEntityDefinitionList(kind string, page int32, size int32, total int32, items []EntityDefinition, ) *EntityDefinitionList`

NewEntityDefinitionList instantiates a new EntityDefinitionList object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewEntityDefinitionListWithDefaults

`func NewEntityDefinitionListWithDefaults() *EntityDefinitionList`

NewEntityDefinitionListWithDefaults instantiates a new EntityDefinitionList object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetKind

`func (o *EntityDefinitionList) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *EntityDefinitionList) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *EntityDefinitionList) SetKind(v string)`

SetKind sets Kind field to given value.


### GetPage

`func (o *EntityDefinitionList) GetPage() int32`

GetPage returns the Page field if non-nil, zero value otherwise.

### GetPageOk

`func (o *EntityDefinitionList) GetPageOk() (*int32, bool)`

GetPageOk returns a tuple with the Page field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPage

`func (o *EntityDefinitionList) SetPage(v int32)`

SetPage sets Page field to given value.


### GetSize

`func (o *EntityDefinitionList) GetSize() int32`

GetSize returns the Size field if non-nil, zero value otherwise.

### GetSizeOk

`func (o *EntityDefinitionList) GetSizeOk() (*int32, bool)`

GetSizeOk returns a tuple with the Size field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSize

`func (o *EntityDefinitionList) SetSize(v int32)`

SetSize sets Size field to given value.


### GetTotal

`func (o *EntityDefinitionList) GetTotal() int32`

GetTotal returns the Total field if non-nil, zero value otherwise.

### GetTotalOk

`func (o *EntityDefinitionList) GetTotalOk() (*int32, bool)`

GetTotalOk returns a tuple with the Total field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTotal

`func (o *EntityDefinitionList) SetTotal(v int32)`

SetTotal sets Total field to given value.


### GetItems

`func (o *EntityDefinitionList) GetItems() []EntityDefinition`

GetItems returns the Items field if non-nil, zero value otherwise.

### GetItemsOk

`func (o *EntityDefinitionList) GetItemsOk() (*[]EntityDefinition, bool)`

GetItemsOk returns a tuple with the Items field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetItems

`func (o *EntityDefinitionList) SetItems(v []EntityDefinition)`

SetItems sets Items field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


