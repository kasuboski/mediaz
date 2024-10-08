// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kasuboski/mediaz/pkg/storage (interfaces: Storage)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_storage.go github.com/kasuboski/mediaz/pkg/storage Storage
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	storage "github.com/kasuboski/mediaz/pkg/storage"
	model "github.com/kasuboski/mediaz/pkg/storage/sqlite/schema/gen/model"
	gomock "go.uber.org/mock/gomock"
)

// MockStorage is a mock of Storage interface.
type MockStorage struct {
	ctrl     *gomock.Controller
	recorder *MockStorageMockRecorder
}

// MockStorageMockRecorder is the mock recorder for MockStorage.
type MockStorageMockRecorder struct {
	mock *MockStorage
}

// NewMockStorage creates a new mock instance.
func NewMockStorage(ctrl *gomock.Controller) *MockStorage {
	mock := &MockStorage{ctrl: ctrl}
	mock.recorder = &MockStorageMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockStorage) EXPECT() *MockStorageMockRecorder {
	return m.recorder
}

// CreateIndexer mocks base method.
func (m *MockStorage) CreateIndexer(arg0 context.Context, arg1 model.Indexer) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateIndexer", arg0, arg1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateIndexer indicates an expected call of CreateIndexer.
func (mr *MockStorageMockRecorder) CreateIndexer(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateIndexer", reflect.TypeOf((*MockStorage)(nil).CreateIndexer), arg0, arg1)
}

// CreateMovie mocks base method.
func (m *MockStorage) CreateMovie(arg0 context.Context, arg1 model.Movie) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateMovie", arg0, arg1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateMovie indicates an expected call of CreateMovie.
func (mr *MockStorageMockRecorder) CreateMovie(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateMovie", reflect.TypeOf((*MockStorage)(nil).CreateMovie), arg0, arg1)
}

// CreateMovieFile mocks base method.
func (m *MockStorage) CreateMovieFile(arg0 context.Context, arg1 model.MovieFile) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateMovieFile", arg0, arg1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateMovieFile indicates an expected call of CreateMovieFile.
func (mr *MockStorageMockRecorder) CreateMovieFile(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateMovieFile", reflect.TypeOf((*MockStorage)(nil).CreateMovieFile), arg0, arg1)
}

// CreateQualityProfileItem mocks base method.
func (m *MockStorage) CreateQualityProfileItem(arg0 context.Context, arg1 model.QualityProfileItem) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateQualityProfileItem", arg0, arg1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateQualityProfileItem indicates an expected call of CreateQualityProfileItem.
func (mr *MockStorageMockRecorder) CreateQualityProfileItem(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateQualityProfileItem", reflect.TypeOf((*MockStorage)(nil).CreateQualityProfileItem), arg0, arg1)
}

// CreateQualityDefinition mocks base method.
func (m *MockStorage) CreateQualityDefinition(arg0 context.Context, arg1 model.QualityDefinition) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateQualityDefinition", arg0, arg1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateQualityDefinition indicates an expected call of CreateQualityDefinition.
func (mr *MockStorageMockRecorder) CreateQualityDefinition(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateQualityDefinition", reflect.TypeOf((*MockStorage)(nil).CreateQualityDefinition), arg0, arg1)
}

// CreateQualityProfile mocks base method.
func (m *MockStorage) CreateQualityProfile(arg0 context.Context, arg1 model.QualityProfile) (int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateQualityProfile", arg0, arg1)
	ret0, _ := ret[0].(int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateQualityProfile indicates an expected call of CreateQualityProfile.
func (mr *MockStorageMockRecorder) CreateQualityProfile(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateQualityProfile", reflect.TypeOf((*MockStorage)(nil).CreateQualityProfile), arg0, arg1)
}

// DeleteIndexer mocks base method.
func (m *MockStorage) DeleteIndexer(arg0 context.Context, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteIndexer", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteIndexer indicates an expected call of DeleteIndexer.
func (mr *MockStorageMockRecorder) DeleteIndexer(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteIndexer", reflect.TypeOf((*MockStorage)(nil).DeleteIndexer), arg0, arg1)
}

// DeleteMovie mocks base method.
func (m *MockStorage) DeleteMovie(arg0 context.Context, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteMovie", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteMovie indicates an expected call of DeleteMovie.
func (mr *MockStorageMockRecorder) DeleteMovie(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteMovie", reflect.TypeOf((*MockStorage)(nil).DeleteMovie), arg0, arg1)
}

// DeleteMovieFile mocks base method.
func (m *MockStorage) DeleteMovieFile(arg0 context.Context, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteMovieFile", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteMovieFile indicates an expected call of DeleteMovieFile.
func (mr *MockStorageMockRecorder) DeleteMovieFile(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteMovieFile", reflect.TypeOf((*MockStorage)(nil).DeleteMovieFile), arg0, arg1)
}

// DeleteQualityProfileItem mocks base method.
func (m *MockStorage) DeleteQualityProfileItem(arg0 context.Context, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteQualityProfileItem", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteQualityProfileItem indicates an expected call of DeleteQualityProfileItem.
func (mr *MockStorageMockRecorder) DeleteQualityProfileItem(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteQualityProfileItem", reflect.TypeOf((*MockStorage)(nil).DeleteQualityProfileItem), arg0, arg1)
}

// DeleteQualityDefinition mocks base method.
func (m *MockStorage) DeleteQualityDefinition(arg0 context.Context, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteQualityDefinition", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteQualityDefinition indicates an expected call of DeleteQualityDefinition.
func (mr *MockStorageMockRecorder) DeleteQualityDefinition(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteQualityDefinition", reflect.TypeOf((*MockStorage)(nil).DeleteQualityDefinition), arg0, arg1)
}

// DeleteQualityProfile mocks base method.
func (m *MockStorage) DeleteQualityProfile(arg0 context.Context, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteQualityProfile", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteQualityProfile indicates an expected call of DeleteQualityProfile.
func (mr *MockStorageMockRecorder) DeleteQualityProfile(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteQualityProfile", reflect.TypeOf((*MockStorage)(nil).DeleteQualityProfile), arg0, arg1)
}

// GetQualityProfileItem mocks base method.
func (m *MockStorage) GetQualityProfileItem(arg0 context.Context, arg1 int64) (model.QualityProfileItem, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetQualityProfileItem", arg0, arg1)
	ret0, _ := ret[0].(model.QualityProfileItem)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetQualityProfileItem indicates an expected call of GetQualityProfileItem.
func (mr *MockStorageMockRecorder) GetQualityProfileItem(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetQualityProfileItem", reflect.TypeOf((*MockStorage)(nil).GetQualityProfileItem), arg0, arg1)
}

// GetQualityDefinition mocks base method.
func (m *MockStorage) GetQualityDefinition(arg0 context.Context, arg1 int64) (model.QualityDefinition, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetQualityDefinition", arg0, arg1)
	ret0, _ := ret[0].(model.QualityDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetQualityDefinition indicates an expected call of GetQualityDefinition.
func (mr *MockStorageMockRecorder) GetQualityDefinition(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetQualityDefinition", reflect.TypeOf((*MockStorage)(nil).GetQualityDefinition), arg0, arg1)
}

// GetQualityProfile mocks base method.
func (m *MockStorage) GetQualityProfile(arg0 context.Context, arg1 int64) (storage.QualityProfile, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetQualityProfile", arg0, arg1)
	ret0, _ := ret[0].(storage.QualityProfile)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetQualityProfile indicates an expected call of GetQualityProfile.
func (mr *MockStorageMockRecorder) GetQualityProfile(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetQualityProfile", reflect.TypeOf((*MockStorage)(nil).GetQualityProfile), arg0, arg1)
}

// Init mocks base method.
func (m *MockStorage) Init(arg0 context.Context, arg1 ...string) error {
	m.ctrl.T.Helper()
	varargs := []any{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Init", varargs...)
	ret0, _ := ret[0].(error)
	return ret0
}

// Init indicates an expected call of Init.
func (mr *MockStorageMockRecorder) Init(arg0 any, arg1 ...any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]any{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Init", reflect.TypeOf((*MockStorage)(nil).Init), varargs...)
}

// ListIndexers mocks base method.
func (m *MockStorage) ListIndexers(arg0 context.Context) ([]*model.Indexer, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListIndexers", arg0)
	ret0, _ := ret[0].([]*model.Indexer)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListIndexers indicates an expected call of ListIndexers.
func (mr *MockStorageMockRecorder) ListIndexers(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListIndexers", reflect.TypeOf((*MockStorage)(nil).ListIndexers), arg0)
}

// ListMovieFiles mocks base method.
func (m *MockStorage) ListMovieFiles(arg0 context.Context) ([]*model.MovieFile, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListMovieFiles", arg0)
	ret0, _ := ret[0].([]*model.MovieFile)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListMovieFiles indicates an expected call of ListMovieFiles.
func (mr *MockStorageMockRecorder) ListMovieFiles(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListMovieFiles", reflect.TypeOf((*MockStorage)(nil).ListMovieFiles), arg0)
}

// ListMovies mocks base method.
func (m *MockStorage) ListMovies(arg0 context.Context) ([]*model.Movie, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListMovies", arg0)
	ret0, _ := ret[0].([]*model.Movie)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListMovies indicates an expected call of ListMovies.
func (mr *MockStorageMockRecorder) ListMovies(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListMovies", reflect.TypeOf((*MockStorage)(nil).ListMovies), arg0)
}

// ListQualityProfileItems mocks base method.
func (m *MockStorage) ListQualityProfileItems(arg0 context.Context) ([]*model.QualityProfileItem, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListQualityProfileItems", arg0)
	ret0, _ := ret[0].([]*model.QualityProfileItem)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListQualityProfileItems indicates an expected call of ListQualityProfileItems.
func (mr *MockStorageMockRecorder) ListQualityProfileItems(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListQualityProfileItems", reflect.TypeOf((*MockStorage)(nil).ListQualityProfileItems), arg0)
}

// ListQualityDefinitions mocks base method.
func (m *MockStorage) ListQualityDefinitions(arg0 context.Context) ([]*model.QualityDefinition, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListQualityDefinitions", arg0)
	ret0, _ := ret[0].([]*model.QualityDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListQualityDefinitions indicates an expected call of ListQualityDefinitions.
func (mr *MockStorageMockRecorder) ListQualityDefinitions(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListQualityDefinitions", reflect.TypeOf((*MockStorage)(nil).ListQualityDefinitions), arg0)
}

// ListQualityProfiles mocks base method.
func (m *MockStorage) ListQualityProfiles(arg0 context.Context) ([]*storage.QualityProfile, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListQualityProfiles", arg0)
	ret0, _ := ret[0].([]*storage.QualityProfile)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListQualityProfiles indicates an expected call of ListQualityProfiles.
func (mr *MockStorageMockRecorder) ListQualityProfiles(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListQualityProfiles", reflect.TypeOf((*MockStorage)(nil).ListQualityProfiles), arg0)
}
