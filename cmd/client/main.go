package main

import (
	"flag"
	"fmt"

	"github.com/rs/zerolog/log"
)

var (
	mainServiceLocation = flag.String("url", "http://localhost:8081", "Main service URL")
	method              = flag.String("method", "ListUsers", "gRPC method to be executed")
	nickname            = flag.String("nickname", "", "Nickname of the user")
	email               = flag.String("email", "", "Email address of the user")
)

func main() {
	flag.Parse()
	var err error
	var resp string
	switch *method {
	case "AddUser":
		resp, err = addUserCall(*nickname, *email, *mainServiceLocation)
	case "DeleteUser":
		resp, err = deleteUserCall(*nickname, *mainServiceLocation)
	case "ListUsers":
		resp, err = listUsersCall(*mainServiceLocation)
	default:
		err = fmt.Errorf("Unknown method.")
	}
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to call method %q", *method)
	}
	fmt.Println(resp)
}
