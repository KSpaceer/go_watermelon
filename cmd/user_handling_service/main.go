package main

import (
	"flag"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KSpaceer/go_watermelon/internal/data"
	pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
	uhs "github.com/KSpaceer/go_watermelon/internal/user_handling/server"
	"github.com/Shopify/sarama"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	timeoutStep     time.Duration = 500 * time.Millisecond
	connectAttempts               = 4
)

var (
	grpcServerEndpoint  = flag.String("grpc-server-endpoint", ":9090", "gRPC server endpoint")
	redisAddr           = flag.String("redis-address", "redis:6379", "Redis DB address")
	pgsInfoFilePath     = flag.String("pgs-info-file", "./pgsinfo.txt", "Postgres info file")
	messageBrokersAddrs = flag.String("brokers-addresses", "kafka-1:9092,kafka-2:9092", "Message brokers addresses")
)

func createDataHandler() (data.Data, error) {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Connecting to database and cache...")
		dataHandler, err := data.NewPGSRedisData(*redisAddr, *pgsInfoFilePath)
		if err == nil {
			log.Info().Msg("Successfully connected to database and cache.")
			return dataHandler, nil
		}
		log.Error().Err(err).Msg("Occured while attempting to connect to database and cache.")
		time.Sleep(timeout)
		timeout += timeoutStep
	}
	return nil, err
}

func createMBProducer(addrs []string, conf *sarama.Config) (sarama.SyncProducer, error) {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Connecting to message broker...")
		var producer sarama.SyncProducer
		producer, err = sarama.NewSyncProducer(addrs, conf)
		if err == nil {
			log.Info().Msg("Successfully connected to message broker.")
			return producer, nil
		}
		log.Error().Err(err).Msg("Occured while attempting to connect to message broker.")
		time.Sleep(timeout)
		timeout += timeoutStep
	}
	return nil, err
}

func main() {
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	dataHandler, err := createDataHandler()
	if err != nil {
		log.Fatal().Err(err).Msg("All attempts to connect to database and cache have failed.")
	}
	defer dataHandler.Disconnect()

	producerConf := sarama.NewConfig()
	producerConf.Producer.Return.Successes = true
	producerConf.Version = sarama.V3_2_0_0
	mbProducer, err := createMBProducer(strings.Split(*messageBrokersAddrs, ","), producerConf)
	if err != nil {
		log.Fatal().Err(err).Msg("All attempts to connect to message broker have failed.")
	}
	defer mbProducer.Close()

	uhServer := uhs.NewUserHandlingServer(dataHandler, mbProducer)
	uhServer.Info().Msg("Created a new UserHandlingServer instance.")

	lis, err := net.Listen("tcp", *grpcServerEndpoint)
	if err != nil {
		uhServer.Error().Msgf("Occured while creating a listener: %v", err)
		log.Fatal().Err(err).Msg("Occured while creating a listener.")
	}
	uhServer.Info().Msg("Listening and ready to serve.")

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
