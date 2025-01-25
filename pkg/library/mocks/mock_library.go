// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/kasuboski/mediaz/pkg/library (interfaces: Library)
//
// Generated by this command:
//
//	mockgen -package mocks -destination mocks/mock_library.go github.com/kasuboski/mediaz/pkg/library Library
//

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	library "github.com/kasuboski/mediaz/pkg/library"
	gomock "go.uber.org/mock/gomock"
)

// MockLibrary is a mock of Library interface.
type MockLibrary struct {
	ctrl     *gomock.Controller
	recorder *MockLibraryMockRecorder
}

// MockLibraryMockRecorder is the mock recorder for MockLibrary.
type MockLibraryMockRecorder struct {
	mock *MockLibrary
}

// NewMockLibrary creates a new mock instance.
func NewMockLibrary(ctrl *gomock.Controller) *MockLibrary {
	mock := &MockLibrary{ctrl: ctrl}
	mock.recorder = &MockLibraryMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockLibrary) EXPECT() *MockLibraryMockRecorder {
	return m.recorder
}

// AddMovie mocks base method.
func (m *MockLibrary) AddMovie(arg0 context.Context, arg1, arg2 string) (library.MovieFile, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AddMovie", arg0, arg1, arg2)
	ret0, _ := ret[0].(library.MovieFile)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// AddMovie indicates an expected call of AddMovie.
func (mr *MockLibraryMockRecorder) AddMovie(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AddMovie", reflect.TypeOf((*MockLibrary)(nil).AddMovie), arg0, arg1, arg2)
}

// FindEpisodes mocks base method.
func (m *MockLibrary) FindEpisodes(arg0 context.Context) ([]string, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindEpisodes", arg0)
	ret0, _ := ret[0].([]string)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindEpisodes indicates an expected call of FindEpisodes.
func (mr *MockLibraryMockRecorder) FindEpisodes(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindEpisodes", reflect.TypeOf((*MockLibrary)(nil).FindEpisodes), arg0)
}

// FindMovies mocks base method.
func (m *MockLibrary) FindMovies(arg0 context.Context) ([]library.MovieFile, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FindMovies", arg0)
	ret0, _ := ret[0].([]library.MovieFile)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FindMovies indicates an expected call of FindMovies.
func (mr *MockLibraryMockRecorder) FindMovies(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FindMovies", reflect.TypeOf((*MockLibrary)(nil).FindMovies), arg0)
}
