// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kasuboski/mediaz/pkg/download (interfaces: DownloadClient)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_download_client.go github.com/kasuboski/mediaz/pkg/download DownloadClient
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	download "github.com/kasuboski/mediaz/pkg/download"
	gomock "go.uber.org/mock/gomock"
)

// MockDownloadClient is a mock of DownloadClient interface.
type MockDownloadClient struct {
	ctrl     *gomock.Controller
	recorder *MockDownloadClientMockRecorder
}

// MockDownloadClientMockRecorder is the mock recorder for MockDownloadClient.
type MockDownloadClientMockRecorder struct {
	mock *MockDownloadClient
}

// NewMockDownloadClient creates a new mock instance.
func NewMockDownloadClient(ctrl *gomock.Controller) *MockDownloadClient {
	mock := &MockDownloadClient{ctrl: ctrl}
	mock.recorder = &MockDownloadClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDownloadClient) EXPECT() *MockDownloadClientMockRecorder {
	return m.recorder
}

// Add mocks base method.
func (m *MockDownloadClient) Add(arg0 context.Context, arg1 download.AddRequest) (download.Status, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Add", arg0, arg1)
	ret0, _ := ret[0].(download.Status)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Add indicates an expected call of Add.
func (mr *MockDownloadClientMockRecorder) Add(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Add", reflect.TypeOf((*MockDownloadClient)(nil).Add), arg0, arg1)
}

// Get mocks base method.
func (m *MockDownloadClient) Get(arg0 context.Context, arg1 download.GetRequest) (download.Status, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(download.Status)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockDownloadClientMockRecorder) Get(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockDownloadClient)(nil).Get), arg0, arg1)
}

// List mocks base method.
func (m *MockDownloadClient) List(arg0 context.Context) ([]download.Status, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].([]download.Status)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockDownloadClientMockRecorder) List(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockDownloadClient)(nil).List), arg0)
}