package main

import (
	"context"
	"flag"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	es "github.com/KSpaceer/go_watermelon/internal/email/server"
)

var (
	emailInfoFilePath   = flag.String("email-info-file", "./emailinfo.csv", "Email info file")
	mainServiceLocation = flag.String("main-service-location", "localhost:8081", "Main service URL")
    imageDirectory = flag.String("image-directory", "/img", "Image directory")
	messageBrokersAddrs = flag.String("brokers-addresses", "localhost:29092,localhost:29093", "Message brokers addresses")
)

func main() {
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	eServer, err := es.NewEmailServer(*emailInfoFilePath, *mainServiceLocation, *imageDirectory, strings.Split(*messageBrokersAddrs, ","))
	if err != nil {
		log.Fatal().Err(err).Msg("Occured while creating a new EmailServer instance")
	}
	defer eServer.Disconnect()
	err = eServer.SubscribeToTopics(context.Background())
	eServer.Wait()
	eServer.Fatal().Msgf("Failed to consume messages: %v", err)
}
