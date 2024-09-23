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
func (m *MockStorage) ListQualityProfiles(arg0 context.Context) ([]storage.QualityProfile, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListQualityProfiles", arg0)
	ret0, _ := ret[0].([]storage.QualityProfile)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListQualityProfiles indicates an expected call of ListQualityProfiles.
func (mr *MockStorageMockRecorder) ListQualityProfiles(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListQualityProfiles", reflect.TypeOf((*MockStorage)(nil).ListQualityProfiles), arg0)
}
