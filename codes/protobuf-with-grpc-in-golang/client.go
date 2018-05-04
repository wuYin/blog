package main

import (
	"google.golang.org/grpc"
	"github.com/labstack/gommon/log"
	pb "blog/proto"
	"context"
	"fmt"
)

func main() {
	conn, err := grpc.Dial(":2333", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("dial error: %v\n", err)
	}
	defer conn.Close()

	client := pb.NewUserInfoServiceClient(conn)

	// 调用服务
	req := new(pb.UserRequest)
	req.Name = "wuYin"
	resp, err := client.GetUserInfo(context.Background(), req)
	if err != nil {
		log.Fatalf("resp error: %v\n", err)
	}

	fmt.Printf("Recevied: %v\n", resp)
}
