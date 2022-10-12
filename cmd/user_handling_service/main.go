package main

import (
	"flag"
	"net"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KSpaceer/go_watermelon/internal/data"
	pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
	uhs "github.com/KSpaceer/go_watermelon/internal/user_handling/server"
	"github.com/Shopify/sarama"
	"github.com/rs/zerolog/log"
)

var (
	grpcServerEndpoint  = flag.String("grpc-server-endpoint", "localhost:9090", "gRPC server endpoint")
	redisAddr           = flag.String("redis-address", "localhost:6379", "Redis DB address")
	pgsInfoFilePath     = flag.String("pgs-info-file", "./pgsinfo.txt", "Postgres info file")
	messageBrokersAddrs = flag.String("brokers-addresses", "localhost:29092,localhost:29093", "Message brokers addresses")
)

func main() {
	flag.Parse()

	dataHandler, err := data.NewPGSRedisData(*redisAddr, *pgsInfoFilePath)
	if err != nil {
		log.Fatal().Err(err).Msg("Occured while creating a dataHandler.")
	}

	mbProducer, err := sarama.NewSyncProducer(strings.Split(*messageBrokersAddrs, ","), sarama.NewConfig())
	if err != nil {
		log.Fatal().Err(err).Msg("Occured while creating a message broker producer.")
	}

	uhServer := uhs.NewUserHandlingServer(dataHandler, mbProducer)
	uhServer.Info().Msg("Created a new UserHandlingServer instance.")
	defer uhServer.Disconnect()

	lis, err := net.Listen("tcp", *grpcServerEndpoint)
	if err != nil {
		log.Fatal().Err(err).Msg("Occured while creating a listener.")
	}

	grpcServer := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	pb.RegisterUserHandlingServer(grpcServer, uhServer)

	cancelChan := make(chan struct{})
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go uhServer.DailyDelivery(wg, cancelChan)

	err = grpcServer.Serve(lis)
	close(cancelChan)
	wg.Wait()
	uhServer.Fatal().Msgf("Occured while serving grpc connection: %v", err)
}
