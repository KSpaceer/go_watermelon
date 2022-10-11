package uh_server

import (
    "context"
    "fmt"
    "strings"
    "sync"
    "time"
    "io"
    "os"
    "net/mail"

    "google.golang.org/protobuf/types/known/emptypb"
    
    "github.com/Shopify/sarama"
    "github.com/rs/zerolog"
    pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
    "github.com/KSpaceer/go_watermelon/internal/data"
    "github.com/KSpaceer/go_watermelon/internal/kafkawriter"
    sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"
)

const (
    deliveryHour = 12
    deliveryMinute = 0
    deliverySecond = 0
    deliveryInterval time.Duration = 24 * time.Hour

    ctxTimeout time.Duration = 3 * time.Second
)

type UserHandlingServer struct {
    pb.UnimplementedUserHandlingServer
    data.Data
    sarama.SyncProducer
    zerolog.Logger
}

func NewUserHandlingServer(dataHandler data.Data, producer sarama.SyncProducer) (*UserHandlingServer) {
    logger := zerolog.New(io.MultiWriter(os.Stderr, kafkawriter.New(producer))).With().Timestamp().Logger()
    return &UserHandlingServer{Data: dataHandler, SyncProducer: producer, Logger: logger}
}

func (s *UserHandlingServer) Disconnect() {
    s.Info().Msg("User handling server is disconnected from DB and MB.")
    s.Data.Disconnect()
    s.SyncProducer.Close()
}

func (s *UserHandlingServer) AuthUser(ctx context.Context, key *pb.Key) (*pb.Response, error) {
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
    return &pb.Response{Message: fmt.Sprintf("Method %s was executed successfully.", operation.Method)}, nil
}

func (s *UserHandlingServer) AddUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
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
    return &pb.Response{Message: "Auth email is sent."}, nil
}

func (s *UserHandlingServer) DeleteUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
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
    err = s.sendAuthEmail(user.Email, key,  "DELETE")
    if err != nil {
        s.Error().Msgf("An error occured while sending message to MB: %v", err)
        return nil, err
    }
    return &pb.Response{Message: "Auth email is sent."}, nil
}

func (s *UserHandlingServer) ListUsers(_ *emptypb.Empty, stream pb.UserHandling_ListUsersServer) error {
    ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
    usersList, err := s.GetUsersFromDatabase(ctx)
    cancel()
    if err != nil {
        s.Error().Msgf("An error occured while executing database operation: %v", err)
        return err
    }
    for _, user := range usersList {
        if err := stream.Send(&pb.User{Nickname: user.Nickname, Email: user.Email}); err != nil {
            return err
        }
    }
    return nil
}

func (s *UserHandlingServer) sendAuthEmail(authInfo ...string) error {
    msg := &sarama.ProducerMessage{
        Topic: sc.AuthTopic,
        Value: sarama.StringEncoder(strings.Join(authInfo, " ")),
    }
    _, _, err := s.SendMessage(msg)
    return err
}

func (s *UserHandlingServer) sendDailyEmail(user data.User) error {
    msg := &sarama.ProducerMessage{
        Topic: sc.DailyDeliveryTopic,
        Value: sarama.StringEncoder(user.Email + " " + user.Nickname),
    }
    _, _, err := s.SendMessage(msg)
    return err
}

func (s *UserHandlingServer) SendDailyMessagesToAllUsers() {
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

