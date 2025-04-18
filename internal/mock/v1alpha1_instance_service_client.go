// Code generated by mockery. DO NOT EDIT.

package mock

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	v1alpha1 "github.com/spacechunks/explorer/api/instance/v1alpha1"
)

// MockV1alpha1InstanceServiceClient is an autogenerated mock type for the InstanceServiceClient type
type MockV1alpha1InstanceServiceClient struct {
	mock.Mock
}

type MockV1alpha1InstanceServiceClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockV1alpha1InstanceServiceClient) EXPECT() *MockV1alpha1InstanceServiceClient_Expecter {
	return &MockV1alpha1InstanceServiceClient_Expecter{mock: &_m.Mock}
}

// DiscoverInstances provides a mock function with given fields: ctx, in, opts
func (_m *MockV1alpha1InstanceServiceClient) DiscoverInstances(ctx context.Context, in *v1alpha1.DiscoverInstanceRequest, opts ...grpc.CallOption) (*v1alpha1.DiscoverInstanceResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for DiscoverInstances")
	}

	var r0 *v1alpha1.DiscoverInstanceResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.DiscoverInstanceRequest, ...grpc.CallOption) (*v1alpha1.DiscoverInstanceResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.DiscoverInstanceRequest, ...grpc.CallOption) *v1alpha1.DiscoverInstanceResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.DiscoverInstanceResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.DiscoverInstanceRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1alpha1InstanceServiceClient_DiscoverInstances_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DiscoverInstances'
type MockV1alpha1InstanceServiceClient_DiscoverInstances_Call struct {
	*mock.Call
}

// DiscoverInstances is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1alpha1.DiscoverInstanceRequest
//   - opts ...grpc.CallOption
func (_e *MockV1alpha1InstanceServiceClient_Expecter) DiscoverInstances(ctx interface{}, in interface{}, opts ...interface{}) *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call {
	return &MockV1alpha1InstanceServiceClient_DiscoverInstances_Call{Call: _e.mock.On("DiscoverInstances",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call) Run(run func(ctx context.Context, in *v1alpha1.DiscoverInstanceRequest, opts ...grpc.CallOption)) *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1alpha1.DiscoverInstanceRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call) Return(_a0 *v1alpha1.DiscoverInstanceResponse, _a1 error) *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call) RunAndReturn(run func(context.Context, *v1alpha1.DiscoverInstanceRequest, ...grpc.CallOption) (*v1alpha1.DiscoverInstanceResponse, error)) *MockV1alpha1InstanceServiceClient_DiscoverInstances_Call {
	_c.Call.Return(run)
	return _c
}

// GetInstance provides a mock function with given fields: ctx, in, opts
func (_m *MockV1alpha1InstanceServiceClient) GetInstance(ctx context.Context, in *v1alpha1.GetInstanceRequest, opts ...grpc.CallOption) (*v1alpha1.GetInstanceResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for GetInstance")
	}

	var r0 *v1alpha1.GetInstanceResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.GetInstanceRequest, ...grpc.CallOption) (*v1alpha1.GetInstanceResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.GetInstanceRequest, ...grpc.CallOption) *v1alpha1.GetInstanceResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.GetInstanceResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.GetInstanceRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1alpha1InstanceServiceClient_GetInstance_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetInstance'
type MockV1alpha1InstanceServiceClient_GetInstance_Call struct {
	*mock.Call
}

// GetInstance is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1alpha1.GetInstanceRequest
//   - opts ...grpc.CallOption
func (_e *MockV1alpha1InstanceServiceClient_Expecter) GetInstance(ctx interface{}, in interface{}, opts ...interface{}) *MockV1alpha1InstanceServiceClient_GetInstance_Call {
	return &MockV1alpha1InstanceServiceClient_GetInstance_Call{Call: _e.mock.On("GetInstance",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1alpha1InstanceServiceClient_GetInstance_Call) Run(run func(ctx context.Context, in *v1alpha1.GetInstanceRequest, opts ...grpc.CallOption)) *MockV1alpha1InstanceServiceClient_GetInstance_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1alpha1.GetInstanceRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_GetInstance_Call) Return(_a0 *v1alpha1.GetInstanceResponse, _a1 error) *MockV1alpha1InstanceServiceClient_GetInstance_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_GetInstance_Call) RunAndReturn(run func(context.Context, *v1alpha1.GetInstanceRequest, ...grpc.CallOption) (*v1alpha1.GetInstanceResponse, error)) *MockV1alpha1InstanceServiceClient_GetInstance_Call {
	_c.Call.Return(run)
	return _c
}

// ListInstances provides a mock function with given fields: ctx, in, opts
func (_m *MockV1alpha1InstanceServiceClient) ListInstances(ctx context.Context, in *v1alpha1.ListInstancesRequest, opts ...grpc.CallOption) (*v1alpha1.ListInstancesResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ListInstances")
	}

	var r0 *v1alpha1.ListInstancesResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.ListInstancesRequest, ...grpc.CallOption) (*v1alpha1.ListInstancesResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.ListInstancesRequest, ...grpc.CallOption) *v1alpha1.ListInstancesResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.ListInstancesResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.ListInstancesRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1alpha1InstanceServiceClient_ListInstances_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListInstances'
type MockV1alpha1InstanceServiceClient_ListInstances_Call struct {
	*mock.Call
}

// ListInstances is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1alpha1.ListInstancesRequest
//   - opts ...grpc.CallOption
func (_e *MockV1alpha1InstanceServiceClient_Expecter) ListInstances(ctx interface{}, in interface{}, opts ...interface{}) *MockV1alpha1InstanceServiceClient_ListInstances_Call {
	return &MockV1alpha1InstanceServiceClient_ListInstances_Call{Call: _e.mock.On("ListInstances",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1alpha1InstanceServiceClient_ListInstances_Call) Run(run func(ctx context.Context, in *v1alpha1.ListInstancesRequest, opts ...grpc.CallOption)) *MockV1alpha1InstanceServiceClient_ListInstances_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1alpha1.ListInstancesRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_ListInstances_Call) Return(_a0 *v1alpha1.ListInstancesResponse, _a1 error) *MockV1alpha1InstanceServiceClient_ListInstances_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_ListInstances_Call) RunAndReturn(run func(context.Context, *v1alpha1.ListInstancesRequest, ...grpc.CallOption) (*v1alpha1.ListInstancesResponse, error)) *MockV1alpha1InstanceServiceClient_ListInstances_Call {
	_c.Call.Return(run)
	return _c
}

// ReceiveInstanceStatusReports provides a mock function with given fields: ctx, in, opts
func (_m *MockV1alpha1InstanceServiceClient) ReceiveInstanceStatusReports(ctx context.Context, in *v1alpha1.ReceiveInstanceStatusReportsRequest, opts ...grpc.CallOption) (*v1alpha1.ReceiveInstanceStatusReportsResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ReceiveInstanceStatusReports")
	}

	var r0 *v1alpha1.ReceiveInstanceStatusReportsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.ReceiveInstanceStatusReportsRequest, ...grpc.CallOption) (*v1alpha1.ReceiveInstanceStatusReportsResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.ReceiveInstanceStatusReportsRequest, ...grpc.CallOption) *v1alpha1.ReceiveInstanceStatusReportsResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.ReceiveInstanceStatusReportsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.ReceiveInstanceStatusReportsRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ReceiveInstanceStatusReports'
type MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call struct {
	*mock.Call
}

// ReceiveInstanceStatusReports is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1alpha1.ReceiveInstanceStatusReportsRequest
//   - opts ...grpc.CallOption
func (_e *MockV1alpha1InstanceServiceClient_Expecter) ReceiveInstanceStatusReports(ctx interface{}, in interface{}, opts ...interface{}) *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call {
	return &MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call{Call: _e.mock.On("ReceiveInstanceStatusReports",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call) Run(run func(ctx context.Context, in *v1alpha1.ReceiveInstanceStatusReportsRequest, opts ...grpc.CallOption)) *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1alpha1.ReceiveInstanceStatusReportsRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call) Return(_a0 *v1alpha1.ReceiveInstanceStatusReportsResponse, _a1 error) *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call) RunAndReturn(run func(context.Context, *v1alpha1.ReceiveInstanceStatusReportsRequest, ...grpc.CallOption) (*v1alpha1.ReceiveInstanceStatusReportsResponse, error)) *MockV1alpha1InstanceServiceClient_ReceiveInstanceStatusReports_Call {
	_c.Call.Return(run)
	return _c
}

// RunChunk provides a mock function with given fields: ctx, in, opts
func (_m *MockV1alpha1InstanceServiceClient) RunChunk(ctx context.Context, in *v1alpha1.RunChunkRequest, opts ...grpc.CallOption) (*v1alpha1.RunChunkResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for RunChunk")
	}

	var r0 *v1alpha1.RunChunkResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RunChunkRequest, ...grpc.CallOption) (*v1alpha1.RunChunkResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1alpha1.RunChunkRequest, ...grpc.CallOption) *v1alpha1.RunChunkResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1alpha1.RunChunkResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1alpha1.RunChunkRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1alpha1InstanceServiceClient_RunChunk_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RunChunk'
type MockV1alpha1InstanceServiceClient_RunChunk_Call struct {
	*mock.Call
}

// RunChunk is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1alpha1.RunChunkRequest
//   - opts ...grpc.CallOption
func (_e *MockV1alpha1InstanceServiceClient_Expecter) RunChunk(ctx interface{}, in interface{}, opts ...interface{}) *MockV1alpha1InstanceServiceClient_RunChunk_Call {
	return &MockV1alpha1InstanceServiceClient_RunChunk_Call{Call: _e.mock.On("RunChunk",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1alpha1InstanceServiceClient_RunChunk_Call) Run(run func(ctx context.Context, in *v1alpha1.RunChunkRequest, opts ...grpc.CallOption)) *MockV1alpha1InstanceServiceClient_RunChunk_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1alpha1.RunChunkRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_RunChunk_Call) Return(_a0 *v1alpha1.RunChunkResponse, _a1 error) *MockV1alpha1InstanceServiceClient_RunChunk_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1alpha1InstanceServiceClient_RunChunk_Call) RunAndReturn(run func(context.Context, *v1alpha1.RunChunkRequest, ...grpc.CallOption) (*v1alpha1.RunChunkResponse, error)) *MockV1alpha1InstanceServiceClient_RunChunk_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockV1alpha1InstanceServiceClient creates a new instance of MockV1alpha1InstanceServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockV1alpha1InstanceServiceClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockV1alpha1InstanceServiceClient {
	mock := &MockV1alpha1InstanceServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
