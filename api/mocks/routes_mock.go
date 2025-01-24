// Code generated by MockGen. DO NOT EDIT.
// Source: routes.go
//
// Generated by this command:
//
//	mockgen -source=routes.go -destination=mocks/routes_mock.go -package=mocks
//

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gin "github.com/gin-gonic/gin"
	providers "github.com/inference-gateway/inference-gateway/providers"
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

// FetchAllModelsHandler mocks base method.
func (m *MockRouter) FetchAllModelsHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "FetchAllModelsHandler", c)
}

// FetchAllModelsHandler indicates an expected call of FetchAllModelsHandler.
func (mr *MockRouterMockRecorder) FetchAllModelsHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchAllModelsHandler", reflect.TypeOf((*MockRouter)(nil).FetchAllModelsHandler), c)
}

// GenerateProvidersTokenHandler mocks base method.
func (m *MockRouter) GenerateProvidersTokenHandler(c *gin.Context) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "GenerateProvidersTokenHandler", c)
}

// GenerateProvidersTokenHandler indicates an expected call of GenerateProvidersTokenHandler.
func (mr *MockRouterMockRecorder) GenerateProvidersTokenHandler(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GenerateProvidersTokenHandler", reflect.TypeOf((*MockRouter)(nil).GenerateProvidersTokenHandler), c)
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

// ValidateProvider mocks base method.
func (m *MockRouter) ValidateProvider(provider string) (*providers.Provider, bool) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ValidateProvider", provider)
	ret0, _ := ret[0].(*providers.Provider)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// ValidateProvider indicates an expected call of ValidateProvider.
func (mr *MockRouterMockRecorder) ValidateProvider(provider any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ValidateProvider", reflect.TypeOf((*MockRouter)(nil).ValidateProvider), provider)
}
