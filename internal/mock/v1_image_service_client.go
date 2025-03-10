// Code generated by mockery. DO NOT EDIT.

package mock

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// MockV1ImageServiceClient is an autogenerated mock type for the ImageServiceClient type
type MockV1ImageServiceClient struct {
	mock.Mock
}

type MockV1ImageServiceClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockV1ImageServiceClient) EXPECT() *MockV1ImageServiceClient_Expecter {
	return &MockV1ImageServiceClient_Expecter{mock: &_m.Mock}
}

// ImageFsInfo provides a mock function with given fields: ctx, in, opts
func (_m *MockV1ImageServiceClient) ImageFsInfo(ctx context.Context, in *v1.ImageFsInfoRequest, opts ...grpc.CallOption) (*v1.ImageFsInfoResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ImageFsInfo")
	}

	var r0 *v1.ImageFsInfoResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ImageFsInfoRequest, ...grpc.CallOption) (*v1.ImageFsInfoResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ImageFsInfoRequest, ...grpc.CallOption) *v1.ImageFsInfoResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.ImageFsInfoResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.ImageFsInfoRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1ImageServiceClient_ImageFsInfo_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ImageFsInfo'
type MockV1ImageServiceClient_ImageFsInfo_Call struct {
	*mock.Call
}

// ImageFsInfo is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1.ImageFsInfoRequest
//   - opts ...grpc.CallOption
func (_e *MockV1ImageServiceClient_Expecter) ImageFsInfo(ctx interface{}, in interface{}, opts ...interface{}) *MockV1ImageServiceClient_ImageFsInfo_Call {
	return &MockV1ImageServiceClient_ImageFsInfo_Call{Call: _e.mock.On("ImageFsInfo",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1ImageServiceClient_ImageFsInfo_Call) Run(run func(ctx context.Context, in *v1.ImageFsInfoRequest, opts ...grpc.CallOption)) *MockV1ImageServiceClient_ImageFsInfo_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1.ImageFsInfoRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1ImageServiceClient_ImageFsInfo_Call) Return(_a0 *v1.ImageFsInfoResponse, _a1 error) *MockV1ImageServiceClient_ImageFsInfo_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1ImageServiceClient_ImageFsInfo_Call) RunAndReturn(run func(context.Context, *v1.ImageFsInfoRequest, ...grpc.CallOption) (*v1.ImageFsInfoResponse, error)) *MockV1ImageServiceClient_ImageFsInfo_Call {
	_c.Call.Return(run)
	return _c
}

// ImageStatus provides a mock function with given fields: ctx, in, opts
func (_m *MockV1ImageServiceClient) ImageStatus(ctx context.Context, in *v1.ImageStatusRequest, opts ...grpc.CallOption) (*v1.ImageStatusResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ImageStatus")
	}

	var r0 *v1.ImageStatusResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ImageStatusRequest, ...grpc.CallOption) (*v1.ImageStatusResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ImageStatusRequest, ...grpc.CallOption) *v1.ImageStatusResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.ImageStatusResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.ImageStatusRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1ImageServiceClient_ImageStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ImageStatus'
type MockV1ImageServiceClient_ImageStatus_Call struct {
	*mock.Call
}

// ImageStatus is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1.ImageStatusRequest
//   - opts ...grpc.CallOption
func (_e *MockV1ImageServiceClient_Expecter) ImageStatus(ctx interface{}, in interface{}, opts ...interface{}) *MockV1ImageServiceClient_ImageStatus_Call {
	return &MockV1ImageServiceClient_ImageStatus_Call{Call: _e.mock.On("ImageStatus",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1ImageServiceClient_ImageStatus_Call) Run(run func(ctx context.Context, in *v1.ImageStatusRequest, opts ...grpc.CallOption)) *MockV1ImageServiceClient_ImageStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1.ImageStatusRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1ImageServiceClient_ImageStatus_Call) Return(_a0 *v1.ImageStatusResponse, _a1 error) *MockV1ImageServiceClient_ImageStatus_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1ImageServiceClient_ImageStatus_Call) RunAndReturn(run func(context.Context, *v1.ImageStatusRequest, ...grpc.CallOption) (*v1.ImageStatusResponse, error)) *MockV1ImageServiceClient_ImageStatus_Call {
	_c.Call.Return(run)
	return _c
}

// ListImages provides a mock function with given fields: ctx, in, opts
func (_m *MockV1ImageServiceClient) ListImages(ctx context.Context, in *v1.ListImagesRequest, opts ...grpc.CallOption) (*v1.ListImagesResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for ListImages")
	}

	var r0 *v1.ListImagesResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ListImagesRequest, ...grpc.CallOption) (*v1.ListImagesResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.ListImagesRequest, ...grpc.CallOption) *v1.ListImagesResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.ListImagesResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.ListImagesRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1ImageServiceClient_ListImages_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ListImages'
type MockV1ImageServiceClient_ListImages_Call struct {
	*mock.Call
}

// ListImages is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1.ListImagesRequest
//   - opts ...grpc.CallOption
func (_e *MockV1ImageServiceClient_Expecter) ListImages(ctx interface{}, in interface{}, opts ...interface{}) *MockV1ImageServiceClient_ListImages_Call {
	return &MockV1ImageServiceClient_ListImages_Call{Call: _e.mock.On("ListImages",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1ImageServiceClient_ListImages_Call) Run(run func(ctx context.Context, in *v1.ListImagesRequest, opts ...grpc.CallOption)) *MockV1ImageServiceClient_ListImages_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1.ListImagesRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1ImageServiceClient_ListImages_Call) Return(_a0 *v1.ListImagesResponse, _a1 error) *MockV1ImageServiceClient_ListImages_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1ImageServiceClient_ListImages_Call) RunAndReturn(run func(context.Context, *v1.ListImagesRequest, ...grpc.CallOption) (*v1.ListImagesResponse, error)) *MockV1ImageServiceClient_ListImages_Call {
	_c.Call.Return(run)
	return _c
}

// PullImage provides a mock function with given fields: ctx, in, opts
func (_m *MockV1ImageServiceClient) PullImage(ctx context.Context, in *v1.PullImageRequest, opts ...grpc.CallOption) (*v1.PullImageResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for PullImage")
	}

	var r0 *v1.PullImageResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.PullImageRequest, ...grpc.CallOption) (*v1.PullImageResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.PullImageRequest, ...grpc.CallOption) *v1.PullImageResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.PullImageResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.PullImageRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1ImageServiceClient_PullImage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'PullImage'
type MockV1ImageServiceClient_PullImage_Call struct {
	*mock.Call
}

// PullImage is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1.PullImageRequest
//   - opts ...grpc.CallOption
func (_e *MockV1ImageServiceClient_Expecter) PullImage(ctx interface{}, in interface{}, opts ...interface{}) *MockV1ImageServiceClient_PullImage_Call {
	return &MockV1ImageServiceClient_PullImage_Call{Call: _e.mock.On("PullImage",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1ImageServiceClient_PullImage_Call) Run(run func(ctx context.Context, in *v1.PullImageRequest, opts ...grpc.CallOption)) *MockV1ImageServiceClient_PullImage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1.PullImageRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1ImageServiceClient_PullImage_Call) Return(_a0 *v1.PullImageResponse, _a1 error) *MockV1ImageServiceClient_PullImage_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1ImageServiceClient_PullImage_Call) RunAndReturn(run func(context.Context, *v1.PullImageRequest, ...grpc.CallOption) (*v1.PullImageResponse, error)) *MockV1ImageServiceClient_PullImage_Call {
	_c.Call.Return(run)
	return _c
}

// RemoveImage provides a mock function with given fields: ctx, in, opts
func (_m *MockV1ImageServiceClient) RemoveImage(ctx context.Context, in *v1.RemoveImageRequest, opts ...grpc.CallOption) (*v1.RemoveImageResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for RemoveImage")
	}

	var r0 *v1.RemoveImageResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.RemoveImageRequest, ...grpc.CallOption) (*v1.RemoveImageResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.RemoveImageRequest, ...grpc.CallOption) *v1.RemoveImageResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.RemoveImageResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.RemoveImageRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockV1ImageServiceClient_RemoveImage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RemoveImage'
type MockV1ImageServiceClient_RemoveImage_Call struct {
	*mock.Call
}

// RemoveImage is a helper method to define mock.On call
//   - ctx context.Context
//   - in *v1.RemoveImageRequest
//   - opts ...grpc.CallOption
func (_e *MockV1ImageServiceClient_Expecter) RemoveImage(ctx interface{}, in interface{}, opts ...interface{}) *MockV1ImageServiceClient_RemoveImage_Call {
	return &MockV1ImageServiceClient_RemoveImage_Call{Call: _e.mock.On("RemoveImage",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *MockV1ImageServiceClient_RemoveImage_Call) Run(run func(ctx context.Context, in *v1.RemoveImageRequest, opts ...grpc.CallOption)) *MockV1ImageServiceClient_RemoveImage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*v1.RemoveImageRequest), variadicArgs...)
	})
	return _c
}

func (_c *MockV1ImageServiceClient_RemoveImage_Call) Return(_a0 *v1.RemoveImageResponse, _a1 error) *MockV1ImageServiceClient_RemoveImage_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockV1ImageServiceClient_RemoveImage_Call) RunAndReturn(run func(context.Context, *v1.RemoveImageRequest, ...grpc.CallOption) (*v1.RemoveImageResponse, error)) *MockV1ImageServiceClient_RemoveImage_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockV1ImageServiceClient creates a new instance of MockV1ImageServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockV1ImageServiceClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockV1ImageServiceClient {
	mock := &MockV1ImageServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
