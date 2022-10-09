package main

import (
    "context"
    "flag"
    "strings"
    "log"

    es "github.com/KSpaceer/go_watermelon/internal/email/server"
)

var (
    emailInfoFilePath = flag.String("email-info-file", "./emailinfo.csv", "Email info file")
    mainServiceLocation = flag.String("main-service-location", "localhost:8081", "Main service URL")
    messageBrokersAddrs = flag.String("brokers-addresses", "localhost:29092,localhost:29093", "Message brokers addresses")
)

func main() {
    flag.Parse()
    eServer, err := es.NewEmailServer(*emailInfoFilePath, *mainServiceLocation, strings.Split(*messageBrokersAddrs, ","))
    if err != nil {
        log.Fatalf("Failed to create a server instance: %v", err)
    }
    defer eServer.Disconnect()
    log.Fatal(eServer.SubscribeToTopics(context.Background()) 
}

