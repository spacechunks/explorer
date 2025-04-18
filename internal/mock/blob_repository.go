// Code generated by mockery. DO NOT EDIT.

package mock

import (
	context "context"

	blob "github.com/spacechunks/explorer/controlplane/blob"

	mock "github.com/stretchr/testify/mock"
)

// MockBlobRepository is an autogenerated mock type for the Repository type
type MockBlobRepository struct {
	mock.Mock
}

type MockBlobRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *MockBlobRepository) EXPECT() *MockBlobRepository_Expecter {
	return &MockBlobRepository_Expecter{mock: &_m.Mock}
}

// BulkGetBlobs provides a mock function with given fields: ctx, hashes
func (_m *MockBlobRepository) BulkGetBlobs(ctx context.Context, hashes []string) ([]blob.Object, error) {
	ret := _m.Called(ctx, hashes)

	if len(ret) == 0 {
		panic("no return value specified for BulkGetBlobs")
	}

	var r0 []blob.Object
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]blob.Object, error)); ok {
		return rf(ctx, hashes)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []blob.Object); ok {
		r0 = rf(ctx, hashes)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]blob.Object)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, hashes)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockBlobRepository_BulkGetBlobs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BulkGetBlobs'
type MockBlobRepository_BulkGetBlobs_Call struct {
	*mock.Call
}

// BulkGetBlobs is a helper method to define mock.On call
//   - ctx context.Context
//   - hashes []string
func (_e *MockBlobRepository_Expecter) BulkGetBlobs(ctx interface{}, hashes interface{}) *MockBlobRepository_BulkGetBlobs_Call {
	return &MockBlobRepository_BulkGetBlobs_Call{Call: _e.mock.On("BulkGetBlobs", ctx, hashes)}
}

func (_c *MockBlobRepository_BulkGetBlobs_Call) Run(run func(ctx context.Context, hashes []string)) *MockBlobRepository_BulkGetBlobs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]string))
	})
	return _c
}

func (_c *MockBlobRepository_BulkGetBlobs_Call) Return(_a0 []blob.Object, _a1 error) *MockBlobRepository_BulkGetBlobs_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockBlobRepository_BulkGetBlobs_Call) RunAndReturn(run func(context.Context, []string) ([]blob.Object, error)) *MockBlobRepository_BulkGetBlobs_Call {
	_c.Call.Return(run)
	return _c
}

// BulkWriteBlobs provides a mock function with given fields: ctx, objects
func (_m *MockBlobRepository) BulkWriteBlobs(ctx context.Context, objects []blob.Object) error {
	ret := _m.Called(ctx, objects)

	if len(ret) == 0 {
		panic("no return value specified for BulkWriteBlobs")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []blob.Object) error); ok {
		r0 = rf(ctx, objects)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockBlobRepository_BulkWriteBlobs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'BulkWriteBlobs'
type MockBlobRepository_BulkWriteBlobs_Call struct {
	*mock.Call
}

// BulkWriteBlobs is a helper method to define mock.On call
//   - ctx context.Context
//   - objects []blob.Object
func (_e *MockBlobRepository_Expecter) BulkWriteBlobs(ctx interface{}, objects interface{}) *MockBlobRepository_BulkWriteBlobs_Call {
	return &MockBlobRepository_BulkWriteBlobs_Call{Call: _e.mock.On("BulkWriteBlobs", ctx, objects)}
}

func (_c *MockBlobRepository_BulkWriteBlobs_Call) Run(run func(ctx context.Context, objects []blob.Object)) *MockBlobRepository_BulkWriteBlobs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].([]blob.Object))
	})
	return _c
}

func (_c *MockBlobRepository_BulkWriteBlobs_Call) Return(_a0 error) *MockBlobRepository_BulkWriteBlobs_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockBlobRepository_BulkWriteBlobs_Call) RunAndReturn(run func(context.Context, []blob.Object) error) *MockBlobRepository_BulkWriteBlobs_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockBlobRepository creates a new instance of MockBlobRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockBlobRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBlobRepository {
	mock := &MockBlobRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
