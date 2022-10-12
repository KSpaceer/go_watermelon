package main

import (
	"context"
	"flag"
	"net/http"

	"github.com/rs/zerolog/log"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	gw "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
)

var (
	grpcServerEndpoint = flag.String("grpc-server-endpoint", "localhost:9090", "gRPC server endpoint")
	httpServerAddr     = flag.String("http-server-address", "localhost:8081", "HTTP server address")
)

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mux := runtime.NewServeMux()
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	err := gw.RegisterUserHandlingHandlerFromEndpoint(ctx, mux, *grpcServerEndpoint, opts)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't register handler from gRPC endpoint.")
	}
	err = http.ListenAndServe(*httpServerAddr, mux)
	log.Fatal().Err(err).Msg("Failed to listen and serve.")
}
