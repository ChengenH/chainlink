// Code generated by mockery v2.5.1. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// PrometheusBackend is an autogenerated mock type for the PrometheusBackend type
type PrometheusBackend struct {
	mock.Mock
}

// SetMaxUnconfirmedBlocks provides a mock function with given fields: _a0
func (_m *PrometheusBackend) SetMaxUnconfirmedBlocks(_a0 int64) {
	_m.Called(_a0)
}

// SetPipelineRunsQueued provides a mock function with given fields: n
func (_m *PrometheusBackend) SetPipelineRunsQueued(n int) {
	_m.Called(n)
}

// SetPipelineTaskRunsQueued provides a mock function with given fields: n
func (_m *PrometheusBackend) SetPipelineTaskRunsQueued(n int) {
	_m.Called(n)
}

// SetUnconfirmedTransactions provides a mock function with given fields: _a0
func (_m *PrometheusBackend) SetUnconfirmedTransactions(_a0 int64) {
	_m.Called(_a0)
}
