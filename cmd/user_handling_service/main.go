package main

import (
    "flag"
    "strings"
    "log"
    "net"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    "github.com/KSpaceer/go_watermelon/internal/data"
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
    dataHandler, err := data.NewPGSRedisData(*redisAddr, *pgsInfoFilePath) 
    if err != nil {
        log.Fatalf("Failed to create a database handler: %v", err)
    }
    mbProducer, err := sarama.NewSyncProducer(strings.Split(*messageBrokerAddrs, ","), sarama.NewConfig())
    if err != nil {
        log.Fatalf("Failed to create a message brocker producer: %v", err)
    }
    uhServer := uhs.NewUserHandlingServer(dataHandler, mbProducer)
    lis, err := net.Listen("tcp", *grpcServerEndpoint)  
    if err != nil {
        log.Fatalf("Failed to listen: %v", err)
    }
    grpcServer := grpc.NewServer() 
    pb.RegisterUserHandlingServer(grpcServer, uhServer)
    //TODO: add daily delivery
    errChan := make(chan error)
    cancelChan := make(chan struct{})
    go uhServer.DailyDelivery(cancelChan, errChan)
    go func() {
        for {
            log.Error(<-errChan)
        }
    }()
    err = grpcServer.Serve(lis)
    cancelChan <- struct{}{}
    log.Fatal(err)
}
