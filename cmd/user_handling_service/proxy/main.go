package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	usingTLS           = flag.Bool("tls", false, "gRPC connection with TLS")
	privateKeyPath     = flag.String("key", "./cert/key.pem", "Private key for TLS")
	certPath           = flag.String("cert", "./cert/cert.pem", "x509 Certificate for TLS")
	caCertPath         = flag.String("ca", "./cert/ca-cert.pem", "CA certificate trusted by the server")
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
		RootCAs:      certPool,
	}

	return credentials.NewTLS(conf), nil
}

func main() {
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var err error
	creds := insecure.NewCredentials()
	if *usingTLS {
		creds, err = loadTLSCredentials()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create TLS credentials")
		}
	}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(creds)}
	mux := runtime.NewServeMux()
	err = registerGRPCHandler(ctx, mux, opts)
	if err != nil {
		log.Fatal().Err(err).Msg("Can't register handler from gRPC endpoint - all attempts have failed.")
	}
	log.Info().Msg("Now listening.")
    if *usingTLS {
        err = http.ListenAndServeTLS(*httpServerAddr, *certPath, *keyPath, mux)
    } else {
	    err = http.ListenAndServe(*httpServerAddr, mux)
    }
	log.Fatal().Err(err).Msg("Failed to listen and serve.")
}
