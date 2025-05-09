// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             (unknown)
// source: platformd/proxy/v1alpha1/service.proto

package v1alpha1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	ProxyService_CreateListeners_FullMethodName = "/platformd.proxy.v1alpha1.ProxyService/CreateListeners"
	ProxyService_DeleteListeners_FullMethodName = "/platformd.proxy.v1alpha1.ProxyService/DeleteListeners"
)

// ProxyServiceClient is the client API for ProxyService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ProxyServiceClient interface {
	CreateListeners(ctx context.Context, in *CreateListenersRequest, opts ...grpc.CallOption) (*CreateListenersResponse, error)
	DeleteListeners(ctx context.Context, in *DeleteListenersRequest, opts ...grpc.CallOption) (*DeleteListenersResponse, error)
}

type proxyServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewProxyServiceClient(cc grpc.ClientConnInterface) ProxyServiceClient {
	return &proxyServiceClient{cc}
}

func (c *proxyServiceClient) CreateListeners(ctx context.Context, in *CreateListenersRequest, opts ...grpc.CallOption) (*CreateListenersResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CreateListenersResponse)
	err := c.cc.Invoke(ctx, ProxyService_CreateListeners_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *proxyServiceClient) DeleteListeners(ctx context.Context, in *DeleteListenersRequest, opts ...grpc.CallOption) (*DeleteListenersResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeleteListenersResponse)
	err := c.cc.Invoke(ctx, ProxyService_DeleteListeners_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ProxyServiceServer is the server API for ProxyService service.
// All implementations must embed UnimplementedProxyServiceServer
// for forward compatibility.
type ProxyServiceServer interface {
	CreateListeners(context.Context, *CreateListenersRequest) (*CreateListenersResponse, error)
	DeleteListeners(context.Context, *DeleteListenersRequest) (*DeleteListenersResponse, error)
	mustEmbedUnimplementedProxyServiceServer()
}

// UnimplementedProxyServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedProxyServiceServer struct{}

func (UnimplementedProxyServiceServer) CreateListeners(context.Context, *CreateListenersRequest) (*CreateListenersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CreateListeners not implemented")
}
func (UnimplementedProxyServiceServer) DeleteListeners(context.Context, *DeleteListenersRequest) (*DeleteListenersResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteListeners not implemented")
}
func (UnimplementedProxyServiceServer) mustEmbedUnimplementedProxyServiceServer() {}
func (UnimplementedProxyServiceServer) testEmbeddedByValue()                      {}

// UnsafeProxyServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ProxyServiceServer will
// result in compilation errors.
type UnsafeProxyServiceServer interface {
	mustEmbedUnimplementedProxyServiceServer()
}

func RegisterProxyServiceServer(s grpc.ServiceRegistrar, srv ProxyServiceServer) {
	// If the following call pancis, it indicates UnimplementedProxyServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&ProxyService_ServiceDesc, srv)
}

func _ProxyService_CreateListeners_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CreateListenersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProxyServiceServer).CreateListeners(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProxyService_CreateListeners_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProxyServiceServer).CreateListeners(ctx, req.(*CreateListenersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProxyService_DeleteListeners_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteListenersRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProxyServiceServer).DeleteListeners(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ProxyService_DeleteListeners_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ProxyServiceServer).DeleteListeners(ctx, req.(*DeleteListenersRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// ProxyService_ServiceDesc is the grpc.ServiceDesc for ProxyService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ProxyService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "platformd.proxy.v1alpha1.ProxyService",
	HandlerType: (*ProxyServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CreateListeners",
			Handler:    _ProxyService_CreateListeners_Handler,
		},
		{
			MethodName: "DeleteListeners",
			Handler:    _ProxyService_DeleteListeners_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "platformd/proxy/v1alpha1/service.proto",
}
