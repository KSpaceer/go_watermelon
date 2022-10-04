package main

import (
    "flag"
    "strings"
    "log"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
    uhs "github.com/KSpaceer/go_watermelon/internal/user_handling/server"
)

var (
    grpcServerEndpoint = flag.String("grpc-server-endpoint", "localhost:9090", "gRPC server endpoint")
    redisAddr = flag.String("redis-address", "localhost:6379", "Redis DB address")
    pgsInfoFilePath = flag.String("pgs-info-file", "./pgsinfo.txt", "Postgres info file")
    messageBrokersAddrs = flag.String("brokers-addresses", "localhost:29092,localhost:29093", "Message brokers addresses")
)

func main() {
    flag.Parse()
    uhServer, err := uhs.NewUserHandlingServer(*redisAddr, *pgsInfoFilePath, strings.Split(*messageBrokerAddrs, ","))
    if err != nil {
        log.Fatalf("Failed to create a server instance: %v", err) // TODO: replace with advanced logger
    }
    defer uhServer.Disconnect()
    lis, err := net.Listen("tcp", *grpcServerEndpoint)  
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }
    grpcServer := grpc.NewServer() 
    pb.RegisterUserHandlingServer(grpcServer, uhServer)
    log.Fatal(grpcServer.Serve(lis))
}
