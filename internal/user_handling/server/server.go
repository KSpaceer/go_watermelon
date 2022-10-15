package uh_server

import (
	"context"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/KSpaceer/go_watermelon/internal/data"
	"github.com/KSpaceer/go_watermelon/internal/kafkawriter"
	sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"
	pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
	"github.com/Shopify/sarama"
	"github.com/rs/zerolog"
)

const (
	// delivery* consts are used to define the time
	// for sending daily message to users.
	deliveryHour                   = 12
	deliveryMinute                 = 0
	deliverySecond                 = 0
	deliveryInterval time.Duration = 24 * time.Hour

	// ctxTimeout is used to make a context with timeout of given time.
	ctxTimeout time.Duration = 3 * time.Second
)

// UserHandlingServer implements UserHandling gRPC service and also embeds
// additional entities to it work such as data.Data (database and cache),
// message broker (Kafka) producer and logger.
type UserHandlingServer struct {
	pb.UnimplementedUserHandlingServer
	data.Data
	sarama.SyncProducer
	zerolog.Logger
}

// NewUserHandlingServer creates a new UserHandlingServer instance using given data.Data and
// Kafka producer. Also, basing on the producer, it creates a logger which writes simultaneously
// to stderr and message broker.
func NewUserHandlingServer(dataHandler data.Data, producer sarama.SyncProducer) *UserHandlingServer {
	logger := zerolog.New(io.MultiWriter(os.Stderr, kafkawriter.New(producer))).With().Timestamp().Logger()
	return &UserHandlingServer{Data: dataHandler, SyncProducer: producer, Logger: logger}
}

// AuthUser is the part of gRPC service implementation. It authenticates the user and executes
// cached operation, which is accessed through given key.
func (s *UserHandlingServer) AuthUser(ctx context.Context, key *pb.Key) (*pb.Response, error) {
	s.Info().Msgf("Got a call for AuthUser method with key %q", key)
	operation, err := s.GetOperation(ctx, key.Key)
	if err != nil {
		s.Error().Msgf("An error occured while accessing cache: %v", err)
		return nil, err
	}
	if operation.Method == "ADD" {
		err = s.AddUserToDatabase(ctx, operation.User)
	} else if operation.Method == "DELETE" {
		err = s.DeleteUserFromDatabase(ctx, operation.User)
	} else {
		return nil, fmt.Errorf("Wrong key.")
	}
	if err != nil {
		s.Error().Msgf("An error occured while executing database operation: %v", err)
		return nil, err
	}
	s.Info().Msgf("Successfully executed method %s for user %s.", operation.Method, operation.User.Nickname)
	return &pb.Response{Message: fmt.Sprintf("Method %s was executed successfully.", operation.Method)}, nil
}

// AddUser is the part of gRPC service implementation. In case the user with this nickname does not exist,
// the method sends an authenticating email (with help of the email service) using user's email address.
func (s *UserHandlingServer) AddUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
	s.Info().Msgf("Got a call for AddUser method with nickname %q and email %q", user.Nickname, user.Email)
	if ok, err := s.CheckNicknameInDatabase(ctx, user.Nickname); err != nil {
		s.Error().Msgf("An error occured while executing database operation: %v", err)
		return nil, err
	} else if ok {
		return nil, fmt.Errorf("User with this nickname already exists.")
	}
	if _, err := mail.ParseAddress(user.Email); err != nil {
		return nil, fmt.Errorf("Invalid email.")
	}
	key, err := s.SetOperation(ctx, data.User{user.Nickname, user.Email}, "ADD")
	if err != nil {
		s.Error().Msgf("An error occured while accessing cache: %v", err)
		return nil, err
	}
	err = s.sendAuthEmail(user.Email, key, "ADD")
	if err != nil {
		s.Error().Msgf("An error occured while sending message to MB: %v", err)
		return nil, err
	}
	s.Info().Msgf("Got a request to add user %s. The auth email is sent.", user.Nickname)
	return &pb.Response{Message: "Auth email is sent."}, nil
}

// DeleteUser is the part of gRPC service implementation. In case the user with this nickname does exist,
// the method sends an authenticating email (with help of the email service) using user's email address.
func (s *UserHandlingServer) DeleteUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
	s.Info().Msgf("Got a call for DeleteUser method with nickname %q and email %q", user.Nickname, user.Email)
	if ok, err := s.CheckNicknameInDatabase(ctx, user.Nickname); err != nil {
		s.Error().Msgf("An error occured while executing database operation: %v", err)
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("There is no user with such nickname.")
	}
	key, err := s.SetOperation(ctx, data.User{user.Nickname, user.Email}, "DELETE")
	if err != nil {
		s.Error().Msgf("An error occured while accessing cache: %v", err)
		return nil, err
	}
	err = s.sendAuthEmail(user.Email, key, "DELETE")
	if err != nil {
		s.Error().Msgf("An error occured while sending message to MB: %v", err)
		return nil, err
	}
	s.Info().Msgf("Got a request to delete user %s. The auth email is sent.", user.Nickname)
	return &pb.Response{Message: "Auth email is sent."}, nil
}

// ListUsers gets list of all users from database and sends it in streaming way.
func (s *UserHandlingServer) ListUsers(_ *emptypb.Empty, stream pb.UserHandling_ListUsersServer) error {
	s.Info().Msg("Got a call for ListUsers method.")
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	usersList, err := s.GetUsersFromDatabase(ctx)
	cancel()
	if err != nil {
		s.Error().Msgf("An error occured while executing database operation: %v", err)
		return err
	}
	for _, user := range usersList {
		if err := stream.Send(&pb.User{Nickname: user.Nickname, Email: user.Email}); err != nil {
			s.Error().Msgf("An error occured while sending the list of users: %v", err)
			return err
		}
	}
	s.Info().Msg("The users list is successfully sent.")
	return nil
}

// sendAuthEmail sends message with request to deliver a authenticating email to the email service
// through message broker.
func (s *UserHandlingServer) sendAuthEmail(authInfo ...string) error {
	msg := &sarama.ProducerMessage{
		Topic: sc.AuthTopic,
		Value: sarama.StringEncoder(strings.Join(authInfo, " ")),
	}
	_, _, err := s.SendMessage(msg)
	return err
}

// sendDailyEmail sends message with request to deliver the user's daily message to the email service
// through message broker.
func (s *UserHandlingServer) sendDailyEmail(user data.User) error {
	msg := &sarama.ProducerMessage{
		Topic: sc.DailyDeliveryTopic,
		Value: sarama.StringEncoder(user.Email + " " + user.Nickname),
	}
	_, _, err := s.SendMessage(msg)
	return err
}

// SendDailyMessages sends messages to message broker with request of sending email for each user.
func (s *UserHandlingServer) SendDailyMessagesToAllUsers() {
	s.Info().Msg("Starting to send daily messages.")
	ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
	usersList, err := s.GetUsersFromDatabase(ctx)
	cancel()
	if err != nil {
		s.Error().Msgf("An error occured while executing database operation: %v", err)
		return
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(usersList))
	for _, user := range usersList {
		go func(user data.User) {
			defer wg.Done()
			err := s.sendDailyEmail(user)
			if err != nil {
				s.Error().Msgf("An error occured while sending message to MB: %v", err)
			}
		}(user)
	}
	wg.Wait()
}

// DailyDelivery waits for the time of delivery, then sends messages to all users with
// constant period of time.
func (s *UserHandlingServer) DailyDelivery(wg *sync.WaitGroup, cancelChan <-chan struct{}) {
	defer wg.Done()
	curTime := time.Now()
	deliveryTime := time.Date(curTime.Year(), curTime.Month(), curTime.Day(), deliveryHour,
		deliveryMinute, deliverySecond, 0, curTime.Location())
	for deliveryTime.Before(curTime) {
		deliveryTime.Add(deliveryInterval)
	}
	waitTimer := time.NewTimer(deliveryTime.Sub(curTime))
outer:
	for {
		select {
		case <-waitTimer.C:
			break outer
		case <-cancelChan:
			return
		}
	}
	ticker := time.NewTicker(deliveryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.SendDailyMessagesToAllUsers()
		case <-cancelChan:
			return
		}
	}
}
