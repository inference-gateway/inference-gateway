// Code generated by MockGen. DO NOT EDIT.
// Source: routes.go
//
// Generated by this command:
//
//	mockgen -source=routes.go -destination=../tests/mocks/routes.go -package=mocks
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gin "github.com/gin-gonic/gin"
	gomock "go.uber.org/mock/gomock"
)

// MockRouter is a mock of Router interface.
type MockRouter struct {
	ctrl     *gomock.Controller
	recorder *MockRouterMockRecorder
	isgomock struct{}
}

// MockRouterMockRecorder is the mock recorder for MockRouter.
type MockRouterMockRecorder struct {
	mock *MockRouter
}

// NewMockRouter creates a new mock instance.
func NewMockRouter(ctrl *gomock.Controller) *MockRouter {
	mock := &MockRouter{ctrl: ctrl}
	mock.recorder = &MockRouterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRouter) EXPECT() *MockRouterMockRecorder {
	return m.recorder
}

// ChatCompletionsHandler mocks base method.
func (m *MockRouter) ChatCompletionsHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ChatCompletionsHandler", c)
}

// ChatCompletionsHandler indicates an expected call of ChatCompletionsHandler.
func (mr *MockRouterMockRecorder) ChatCompletionsHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ChatCompletionsHandler", reflect.TypeOf((*MockRouter)(nil).ChatCompletionsHandler), c)
}

// CompletionsHandler mocks base method.
func (m *MockRouter) CompletionsHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "CompletionsHandler", c)
}

// CompletionsHandler indicates an expected call of CompletionsHandler.
func (mr *MockRouterMockRecorder) CompletionsHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CompletionsHandler", reflect.TypeOf((*MockRouter)(nil).CompletionsHandler), c)
}

// HealthcheckHandler mocks base method.
func (m *MockRouter) HealthcheckHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "HealthcheckHandler", c)
}

// HealthcheckHandler indicates an expected call of HealthcheckHandler.
func (mr *MockRouterMockRecorder) HealthcheckHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HealthcheckHandler", reflect.TypeOf((*MockRouter)(nil).HealthcheckHandler), c)
}

// ListModelsHandler mocks base method.
func (m *MockRouter) ListModelsHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ListModelsHandler", c)
}

// ListModelsHandler indicates an expected call of ListModelsHandler.
func (mr *MockRouterMockRecorder) ListModelsHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListModelsHandler", reflect.TypeOf((*MockRouter)(nil).ListModelsHandler), c)
}

// NotFoundHandler mocks base method.
func (m *MockRouter) NotFoundHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "NotFoundHandler", c)
}

// NotFoundHandler indicates an expected call of NotFoundHandler.
func (mr *MockRouterMockRecorder) NotFoundHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NotFoundHandler", reflect.TypeOf((*MockRouter)(nil).NotFoundHandler), c)
}

// ProxyHandler mocks base method.
func (m *MockRouter) ProxyHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "ProxyHandler", c)
}

// ProxyHandler indicates an expected call of ProxyHandler.
func (mr *MockRouterMockRecorder) ProxyHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ProxyHandler", reflect.TypeOf((*MockRouter)(nil).ProxyHandler), c)
}
