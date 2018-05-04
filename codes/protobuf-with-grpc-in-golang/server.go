package main

import (
	"context"
	pb "blog/proto"
	"net"
	"google.golang.org/grpc"
	"github.com/labstack/gommon/log"
	"fmt"
)

// 定义服务端实现约定的接口
type UserInfoService struct{}

var u = UserInfoService{}

func (s *UserInfoService) GetUserInfo(ctx context.Context, req *pb.UserRequest) (resp *pb.UserResponse, err error) {
	name := req.Name

	// 在数据库中查找用户信息
	// ...
	if name == "wuYin" {
		resp = &pb.UserResponse{
			Id:    233,
			Name:  name,
			Age:   20,
			Title: []string{"Gopher", "PHPer"}, // repeated 字段是 slice 类型
		}
	}
	err = nil
	return
}

func main() {
	port := ":2333"
	l, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("listen error: %v\n", err)
	}
	fmt.Printf("listen %s\n", port)
	s := grpc.NewServer()
	// 注意第二个参数 UserInfoServiceServer 是接口类型的变量
	// 需要取地址传参
	pb.RegisterUserInfoServiceServer(s, &u)
	s.Serve(l)
}
