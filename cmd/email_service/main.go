package main

import (
	"context"
	"flag"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Shopify/sarama"

	es "github.com/KSpaceer/go_watermelon/internal/email/server"
)

const (
	timeoutStep     time.Duration = 500 * time.Millisecond
	connectAttempts               = 4
)

var (
	emailInfoFilePath   = flag.String("email-info-file", "./emailinfo.csv", "Email info file")
	mainServiceLocation = flag.String("main-service-location", "localhost:8081", "Main service URL")
	imageDirectory      = flag.String("image-directory", "./img", "Image directory")
	messageBrokersAddrs = flag.String("brokers-addresses", "kafka-1:9092,kafka-2:9092", "Message brokers addresses")
)

func createConsumerGroup(addrs []string, conf *sarama.Config) (sarama.ConsumerGroup, error) {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Creating a consumer group in message broker...")
		var consumerGroup sarama.ConsumerGroup
		consumerGroup, err = sarama.NewConsumerGroup(addrs, "emailsend", conf)
		if err == nil {
			log.Info().Msg("Successfully created a consumer group.")
			return consumerGroup, nil
		}
		log.Error().Err(err).Msg("Occured while attempting to create a consumer group.")
		time.Sleep(timeout)
		timeout += timeoutStep
	}
	return nil, err
}

func createLogProducer(addrs []string, conf *sarama.Config) (sarama.SyncProducer, error) {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Creating a sync producer in message broker...")
		var logProducer sarama.SyncProducer
		logProducer, err = sarama.NewSyncProducer(addrs, conf)
		if err == nil {
			log.Info().Msg("Successfully created a sync producer.")
			return logProducer, nil
		}
		log.Error().Err(err).Msg("Occured while attempting to create a sync producer.")
		time.Sleep(timeout)
		timeout += timeoutStep
	}
	return nil, err
}

func main() {
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	addrs := strings.Split(*messageBrokersAddrs, ",")

	conf := sarama.NewConfig()
	conf.Producer.Return.Successes = true
	conf.Producer.Return.Errors = true
	conf.Version = sarama.V3_2_0_0

	consumerGroup, err := createConsumerGroup(addrs, conf)
	if err != nil {
		log.Fatal().Err(err).Msg("All attempts to create a consumer group have failed.")
	}
	defer consumerGroup.Close()

	logProducer, err := createLogProducer(addrs, conf)
	if err != nil {
		log.Fatal().Err(err).Msg("All attempts to create a sync producer have failed.")
	}
	defer logProducer.Close()

	eServer, err := es.NewEmailServer(*emailInfoFilePath, *mainServiceLocation, *imageDirectory, consumerGroup, logProducer)
	if err != nil {
		log.Fatal().Err(err).Msg("Occured while creating a new EmailServer instance")
	}
	err = eServer.SubscribeToTopics(context.Background())
	eServer.Wait()
	eServer.Fatal().Msgf("Failed to consume messages: %v", err)
}
