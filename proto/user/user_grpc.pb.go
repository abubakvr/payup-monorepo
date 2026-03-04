// Code generated for user.proto gRPC. Package userpb.
package userpb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const _ = grpc.SupportPackageIsVersion9

const UserServiceForKYC_GetUserForKYC_FullMethodName = "/user.UserServiceForKYC/GetUserForKYC"

type UserServiceForKYCClient interface {
	GetUserForKYC(ctx context.Context, in *GetUserForKYCRequest, opts ...grpc.CallOption) (*GetUserForKYCResponse, error)
}

type userServiceForKYCClient struct {
	cc grpc.ClientConnInterface
}

func NewUserServiceForKYCClient(cc grpc.ClientConnInterface) UserServiceForKYCClient {
	return &userServiceForKYCClient{cc}
}

func (c *userServiceForKYCClient) GetUserForKYC(ctx context.Context, in *GetUserForKYCRequest, opts ...grpc.CallOption) (*GetUserForKYCResponse, error) {
	out := new(GetUserForKYCResponse)
	err := c.cc.Invoke(ctx, UserServiceForKYC_GetUserForKYC_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type UserServiceForKYCServer interface {
	GetUserForKYC(context.Context, *GetUserForKYCRequest) (*GetUserForKYCResponse, error)
	mustEmbedUnimplementedUserServiceForKYCServer()
}

type UnimplementedUserServiceForKYCServer struct{}

func (UnimplementedUserServiceForKYCServer) GetUserForKYC(context.Context, *GetUserForKYCRequest) (*GetUserForKYCResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method GetUserForKYC not implemented")
}
func (UnimplementedUserServiceForKYCServer) mustEmbedUnimplementedUserServiceForKYCServer() {}

type UnsafeUserServiceForKYCServer interface {
	mustEmbedUnimplementedUserServiceForKYCServer()
}

func RegisterUserServiceForKYCServer(s grpc.ServiceRegistrar, srv UserServiceForKYCServer) {
	s.RegisterService(&UserServiceForKYC_ServiceDesc, srv)
}

func _UserServiceForKYC_GetUserForKYC_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUserForKYCRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UserServiceForKYCServer).GetUserForKYC(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: UserServiceForKYC_GetUserForKYC_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UserServiceForKYCServer).GetUserForKYC(ctx, req.(*GetUserForKYCRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var UserServiceForKYC_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "user.UserServiceForKYC",
	HandlerType: (*UserServiceForKYCServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetUserForKYC",
			Handler:    _UserServiceForKYC_GetUserForKYC_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
}
