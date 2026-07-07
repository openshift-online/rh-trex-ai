# BuildList

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Kind** | **string** |  | 
**Page** | **int32** |  | 
**Size** | **int32** |  | 
**Total** | **int32** |  | 
**Items** | [**[]Build**](Build.md) |  | 

## Methods

### NewBuildList

`func NewBuildList(kind string, page int32, size int32, total int32, items []Build, ) *BuildList`

NewBuildList instantiates a new BuildList object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewBuildListWithDefaults

`func NewBuildListWithDefaults() *BuildList`

NewBuildListWithDefaults instantiates a new BuildList object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetKind

`func (o *BuildList) GetKind() string`

GetKind returns the Kind field if non-nil, zero value otherwise.

### GetKindOk

`func (o *BuildList) GetKindOk() (*string, bool)`

GetKindOk returns a tuple with the Kind field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetKind

`func (o *BuildList) SetKind(v string)`

SetKind sets Kind field to given value.


### GetPage

`func (o *BuildList) GetPage() int32`

GetPage returns the Page field if non-nil, zero value otherwise.

### GetPageOk

`func (o *BuildList) GetPageOk() (*int32, bool)`

GetPageOk returns a tuple with the Page field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetPage

`func (o *BuildList) SetPage(v int32)`

SetPage sets Page field to given value.


### GetSize

`func (o *BuildList) GetSize() int32`

GetSize returns the Size field if non-nil, zero value otherwise.

### GetSizeOk

`func (o *BuildList) GetSizeOk() (*int32, bool)`

GetSizeOk returns a tuple with the Size field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetSize

`func (o *BuildList) SetSize(v int32)`

SetSize sets Size field to given value.


### GetTotal

`func (o *BuildList) GetTotal() int32`

GetTotal returns the Total field if non-nil, zero value otherwise.

### GetTotalOk

`func (o *BuildList) GetTotalOk() (*int32, bool)`

GetTotalOk returns a tuple with the Total field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTotal

`func (o *BuildList) SetTotal(v int32)`

SetTotal sets Total field to given value.


### GetItems

`func (o *BuildList) GetItems() []Build`

GetItems returns the Items field if non-nil, zero value otherwise.

### GetItemsOk

`func (o *BuildList) GetItemsOk() (*[]Build, bool)`

GetItemsOk returns a tuple with the Items field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetItems

`func (o *BuildList) SetItems(v []Build)`

SetItems sets Items field to given value.



[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


