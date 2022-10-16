package main

import (
	"context"
	"flag"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	gw "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
)

const (
	timeoutStep     time.Duration = 500 * time.Millisecond
	connectAttempts               = 4
)

var (
	grpcServerEndpoint = flag.String("grpc-server-endpoint", "mainservice:9090", "gRPC server endpoint")
	httpServerAddr     = flag.String("http-server-address", ":8081", "HTTP server address")
)

func registerGRPCHandler(ctx context.Context, mux *runtime.ServeMux, opts []grpc.DialOption) error {
	var err error
	timeout := timeoutStep
	for i := 0; i < connectAttempts; i++ {
		log.Info().Msg("Trying to register a gRPC handler...")
		err = gw.RegisterUserHandlingHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
		if err == nil {
			log.Info().Msg("Successfully registered a gRPC handler")
			return nil
		}
		log.Error().Err(err).Msg("Occured while attempting to register a gRPC handler.")
		time.Sleep(timeout)
		timeout += timeoutStep
	}
	return err
}

func main() {
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := registerGRPCHandler(ctx, mux, opts)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't register handler from gRPC endpoint - all attempts have failed.")
	}
	log.Info().Msg("Now listening.")
	err = http.ListenAndServe(*httpServerAddr, mux)
	log.Fatal().Err(err).Msg("Failed to listen and serve.")
}
