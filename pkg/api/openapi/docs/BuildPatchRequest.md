# BuildPatchRequest

## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**ProjectId** | Pointer to **string** |  | [optional] 
**Status** | Pointer to **string** |  | [optional] 
**BuildLog** | Pointer to **string** |  | [optional] 
**TriggeredBy** | Pointer to **string** |  | [optional] 
**CompletedAt** | Pointer to **time.Time** |  | [optional] 

## Methods

### NewBuildPatchRequest

`func NewBuildPatchRequest() *BuildPatchRequest`

NewBuildPatchRequest instantiates a new BuildPatchRequest object
This constructor will assign default values to properties that have it defined,
and makes sure properties required by API are set, but the set of arguments
will change when the set of required properties is changed

### NewBuildPatchRequestWithDefaults

`func NewBuildPatchRequestWithDefaults() *BuildPatchRequest`

NewBuildPatchRequestWithDefaults instantiates a new BuildPatchRequest object
This constructor will only assign default values to properties that have it defined,
but it doesn't guarantee that properties required by API are set

### GetProjectId

`func (o *BuildPatchRequest) GetProjectId() string`

GetProjectId returns the ProjectId field if non-nil, zero value otherwise.

### GetProjectIdOk

`func (o *BuildPatchRequest) GetProjectIdOk() (*string, bool)`

GetProjectIdOk returns a tuple with the ProjectId field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetProjectId

`func (o *BuildPatchRequest) SetProjectId(v string)`

SetProjectId sets ProjectId field to given value.

### HasProjectId

`func (o *BuildPatchRequest) HasProjectId() bool`

HasProjectId returns a boolean if a field has been set.

### GetStatus

`func (o *BuildPatchRequest) GetStatus() string`

GetStatus returns the Status field if non-nil, zero value otherwise.

### GetStatusOk

`func (o *BuildPatchRequest) GetStatusOk() (*string, bool)`

GetStatusOk returns a tuple with the Status field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetStatus

`func (o *BuildPatchRequest) SetStatus(v string)`

SetStatus sets Status field to given value.

### HasStatus

`func (o *BuildPatchRequest) HasStatus() bool`

HasStatus returns a boolean if a field has been set.

### GetBuildLog

`func (o *BuildPatchRequest) GetBuildLog() string`

GetBuildLog returns the BuildLog field if non-nil, zero value otherwise.

### GetBuildLogOk

`func (o *BuildPatchRequest) GetBuildLogOk() (*string, bool)`

GetBuildLogOk returns a tuple with the BuildLog field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetBuildLog

`func (o *BuildPatchRequest) SetBuildLog(v string)`

SetBuildLog sets BuildLog field to given value.

### HasBuildLog

`func (o *BuildPatchRequest) HasBuildLog() bool`

HasBuildLog returns a boolean if a field has been set.

### GetTriggeredBy

`func (o *BuildPatchRequest) GetTriggeredBy() string`

GetTriggeredBy returns the TriggeredBy field if non-nil, zero value otherwise.

### GetTriggeredByOk

`func (o *BuildPatchRequest) GetTriggeredByOk() (*string, bool)`

GetTriggeredByOk returns a tuple with the TriggeredBy field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetTriggeredBy

`func (o *BuildPatchRequest) SetTriggeredBy(v string)`

SetTriggeredBy sets TriggeredBy field to given value.

### HasTriggeredBy

`func (o *BuildPatchRequest) HasTriggeredBy() bool`

HasTriggeredBy returns a boolean if a field has been set.

### GetCompletedAt

`func (o *BuildPatchRequest) GetCompletedAt() time.Time`

GetCompletedAt returns the CompletedAt field if non-nil, zero value otherwise.

### GetCompletedAtOk

`func (o *BuildPatchRequest) GetCompletedAtOk() (*time.Time, bool)`

GetCompletedAtOk returns a tuple with the CompletedAt field if it's non-nil, zero value otherwise
and a boolean to check if the value has been set.

### SetCompletedAt

`func (o *BuildPatchRequest) SetCompletedAt(v time.Time)`

SetCompletedAt sets CompletedAt field to given value.

### HasCompletedAt

`func (o *BuildPatchRequest) HasCompletedAt() bool`

HasCompletedAt returns a boolean if a field has been set.


[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


