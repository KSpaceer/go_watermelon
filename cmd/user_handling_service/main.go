package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KSpaceer/go_watermelon/internal/data"
	pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
	uhs "github.com/KSpaceer/go_watermelon/internal/user_handling/server"
	"github.com/Shopify/sarama"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	timeoutStep            time.Duration = 500 * time.Millisecond
	connectAttempts                      = 4
	deliveryTimeEnvVar                   = "GWM_DELIVERY_TIME"
	deliveryIntervalEnvVar               = "GWM_DELIVERY_INTERVAL"
)

var (
	grpcServerEndpoint  = flag.String("grpc-server-endpoint", ":9090", "gRPC server endpoint")
	redisAddr           = flag.String("redis-address", "redis:6379", "Redis DB address")
	pgsInfoFilePath     = flag.String("pgs-info-file", "./pgsinfo.txt", "Postgres info file")
	messageBrokersAddrs = flag.String("brokers-addresses", "kafka-1:9092,kafka-2:9092", "Message brokers addresses")
	usingTLS            = flag.Bool("tls", false, "gRPC connection with TLS")
	privateKeyPath      = flag.String("key", "./cert/key.pem", "Private key for TLS")
	certPath            = flag.String("cert", "./cert/cert.pem", "x509 Certificate for TLS")
	caCertPath          = flag.String("ca", "./cert/ca-cert.pem", "CA certificate trusted by the service")
)

func createRedisCache() (data.Cache, error) {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Connecting to cache...")
		var cache data.Cache
		cache, err = data.NewRedisCache(*redisAddr)
		if err == nil {
			log.Info().Msg("Successfully connected to cache.")
			return cache, nil
		}
		log.Error().Err(err).Msg("Occured while attempting to connect to cache.")
		time.Sleep(timeout)
		timeout += timeoutStep
	}
	return nil, err
}

func createPgsDB() (data.DB, error) {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Connecting to database...")
		var db data.DB
		db, err = data.NewPgsDB(*pgsInfoFilePath)
		if err == nil {
			log.Info().Msg("Successfully connected to database.")
			return db, nil
		}
		log.Error().Err(err).Msg("Occured while attempting to connect to database.")
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

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	caCertPEM, err := os.ReadFile(*caCertPath)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("Failed to add trusted CA certificate")
	}

	cert, err := tls.LoadX509KeyPair(*certPath, *privateKeyPath)
	if err != nil {
		return nil, err
	}
	conf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(conf), nil
}

func main() {
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	cache, err := createRedisCache()
	if err != nil {
		log.Fatal().Err(err).Msg("All attempts to connect to cache have failed.")
	}

	db, err := createPgsDB()
	if err != nil {
		cache.Close()
		log.Fatal().Err(err).Msg("All attempts to connect to database have failed.")
	}

	dataHandler := data.NewData(cache, db)
	defer dataHandler.Disconnect()

	producerConf := sarama.NewConfig()
	producerConf.Producer.Return.Successes = true
	producerConf.Version = sarama.V3_2_0_0
	mbProducer, err := createMBProducer(strings.Split(*messageBrokersAddrs, ","), producerConf)
	if err != nil {
		log.Fatal().Err(err).Msg("All attempts to connect to message broker have failed.")
	}
	defer mbProducer.Close()

	deliveryTime := os.Getenv(deliveryTimeEnvVar)
	if err := uhs.SetDeliveryTime(deliveryTime); err != nil {
		log.Fatal().Err(err).Msg("Couldn't set new delivery time.")
	}
	h, m, s := uhs.GetDeliveryTime()
	log.Info().Msgf("Set delivery time: %d:%d:%d", h, m, s)

	deliveryInterval := os.Getenv(deliveryIntervalEnvVar)
	if err := uhs.SetDeliveryInterval(deliveryInterval); err != nil {
		log.Fatal().Err(err).Msg("Couldn't set new delivery interval.")
	}
	log.Info().Msgf("Set delivery interval: %s", uhs.GetDeliveryInterval())

	uhServer := uhs.NewUserHandlingServer(dataHandler, mbProducer)
	uhServer.Info().Msg("Created a new UserHandlingServer instance.")

	lis, err := net.Listen("tcp", *grpcServerEndpoint)
	if err != nil {
		uhServer.Error().Msgf("Occured while creating a listener: %v", err)
		log.Fatal().Err(err).Msg("Occured while creating a listener.")
	}
	uhServer.Info().Msg("Listening and ready to serve.")

	creds := insecure.NewCredentials()
	if *usingTLS {
		creds, err = loadTLSCredentials()
		if err != nil {
			uhServer.Fatal().Msgf("Failed to create TLS credentials: %v", err)
		}
	}
	grpcServer := grpc.NewServer(grpc.Creds(creds))
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
