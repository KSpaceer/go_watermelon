package uh_server

import (
    "context"
    
    "google.golang.org/grpc"

    "github.com/Shopify/sarama"
    pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto/user_handling_proto"
    "github.com/KSpaceer/go_watermelon/internal/data"
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
    err = s.sendEmail(user.Email, key, "ADD")
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
    err = s.sendEmail(user, key,  "DELETE")
    if err != nil {
        return nil, err
    }
    return &pb.Response{Message: "OK"}, nil
}

func (s *UserHandlingServer) ListUsers(stream pb.UserHandling_ListUsersServer) error {
    usersList, err := s.GetUsersFromDatabase()
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

func (s *UserHandlingServer) sendEmail(email, method string) error {
    msg := &sarama.ProducerMessage{
        Topic: "auth"
        Value: sarama.StringEncoder(email + " " + method)
    }
    _, _, err := s.SendMessage(msg) // TODO: add partition and offset for logging
    return err
}

