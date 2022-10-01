package uh_server

import (
    "context"
    "strings"
    "time"
    
    "google.golang.org/grpc"

    "github.com/Shopify/sarama"
    pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
    "github.com/KSpaceer/go_watermelon/internal/data"
    sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"
)

const (
    deliveryHour := 12
    deliveryMinute := 0
    deliverySecond := 0
    deliveryInterval time.Duration = 24 * time.Hour

    ctxTimeout time.Duration = 3 * time.Second
)

type UserHandlingServer struct {
    pb.UnimplementedUserHandlingServer
    *data.Data
    sarama.SyncProducer
}

func NewUserHandlingServer(redisAddress, pgsInfoFile string, brokersAddresses []string) (*userHandlingServer, error) {
    s := &UserHandlingServer{}
    var err error
    s.Data, err = data.NewData(redisAddress, pgsInfoFile)
    if err != nil {
        return nil, err
    }
    s.SyncProducer, err = sarama.NewSyncProducer(brokersAddresses, sarama.NewConfig())
    if err != nil {
        return nil, err
    }
    return s, nil
}

func (s *UserHandlingServer) AuthUser(ctx context.Context, key *pb.Key) (*pb.Response, error) {
    operation, err := s.GetOperation(ctx, key) 
    if err != nil {
        return nil, err
    }
    if operation.Method == "ADD" {
        s.AddUserToDatabase(ctx, operation.User)
    } else if operation.Method == "DELETE" {
        s.DeleteUserFromDatabase(ctx, operation.User)
    } else {
        return &pb.Response{Message: "Wrong key."}, nil
    }
    return &pb.Response{Message: "OK"}, nil
}

func (s *UserHandlingServer) AddUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
    if ok, err := s.CheckNicknameInDatabase(ctx, user.Nickname); err != nil {
        return nil, err
    } else if ok {
        return &pb.Response{Message: "User with this nickname is already exists."}, nil
    }
    key, err := s.SetOperation(ctx, data.User{user.Nickname, user.Email}, "ADD")
    if err != nil {
        return nil, err
    }
    err = s.sendAuthEmail(user.Email, key, "ADD")
    if err != nil {
        return nil, err
    }
    return &pb.Response{Message: "OK"}, nil
}

func (s *UserHandlingServer) DeleteUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
    if ok, err := s.CheckNicknameInDatabase(ctx, user.Nickname); err != nil {
        return nil, err
    } else if !ok {
        return &pb.Response{Message: "There is no user with such nickname."}, nil
    }
    key, err := s.SetOperation(ctx, data.User{user.Nickname, user.Email}, "DELETE")
    if err != nil {
        return nil, err
    }
    err = s.sendAuthEmail(user, key,  "DELETE")
    if err != nil {
        return nil, err
    }
    return &pb.Response{Message: "OK"}, nil
}

func (s *UserHandlingServer) ListUsers(stream pb.UserHandling_ListUsersServer) error {
    ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
    usersList, err := s.GetUsersFromDatabase(ctx)
    cancel()
    if err != nil {
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
        Value: sarama.StringEncoder(strings.Join(authInfo, " "))
    }
    _, _, err := s.SendMessage(msg) // TODO: add partition and offset for logging
    return err
}

func (s *UserHandlingServer) sendDailyEmail(user data.User) error {
    msg := &sarama.ProducerMessage{
        Topic: sc.DailyDeliveryTopic,
        Value: sarama.StringEncoder(user.Email + " " + user.Nickname)
    }
    _, _, err := s.SendMessage(msg) // TODO: look 6 rows higher
    return err
}
    

func (s *UserHandlingServer) DailyDelivery(errChan chan<- error) {
    curTime := time.Now()
    deliveryTime := time.Date(curTime.Year(), curTime.Month(), curTime.Day(), deliveryHour,
                                deliveryMinute, deliverySecond, 0, curTime.Location())
    for deliveryTime.Before(curTime) {
        deliveryTime.Add(deliveryInterval)
    }
    waitTimer := time.NewTimer(deliveryTime.Sub(curTime))
    <-waitTimer.C
    ticker := time.NewTicker(deliveryInterval)
    defer ticker.Stop()
    for {
        <-ticker.C
        ctx, cancel := context.WithTimeout(context.Background(), ctxTimeout)
        usersList, err := s.GetUsersFromDatabase(ctx)
        cancel()
        if err != nil {
            errChan <- err
        }
        for _, user := range usersList {
            go func(user data.User) {
                err := s.sendDailyEmail(user) 
                if err != nil {
                    errChan <- err
                }
            }(user)
        }
    }
}

