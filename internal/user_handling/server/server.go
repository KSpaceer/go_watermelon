package uh_server

import (
    "context"
    "database/sql"
    
    "google.golang.org/grpc"
    pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto/user_handling_proto"
    "github.com/KSpaceer/go_watermelon/internal/data"
)

type userHandlingServer struct {
    pb.UnimplementedUserHandlingServer
    data.Data
}

func (s *userHandlingServer) AuthUser(ctx context.Context, key *pb.Key) (*pb.Response, error) {
    operation, err := s.GetOperation(key) 
    if err != nil {
        return nil, err
    }
    if operation.Method == "ADD" {
        AddToDatabase(operation.User)
    } else if operation.Method == "DELETE" {
        DeleteFromDatabase(operation.User)
    } else {
        return &pb.Response{Message: "Wrong key."}, nil
    }
    return &pb.Response{Message: "OK"}, nil
}

func (s *userHandlingServer) AddUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
    if ok, err := CheckDatabase(user.Nickname); err != nil {
        return nil, err
    } else if ok {
        return &pb.Response{Message: "User with this nickname is already exists."}, nil
    }
    err = s.SetOperation(user, "ADD")
    if err != nil {
        return nil, err
    }
    SendEmail(user.Nickname, "add")
    return &pb.Response{Message: "OK"}, nil
}

func (s *userHandlingServer) DeleteUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
    if ok, err := CheckDatabase(user.Nickname); err != nil {
        return nil, err
    } else if !ok {
        return &pb.Response{Message: "There is no user with such nickname."}
    }
    SendEmail(user.Nickname, "delete")
    return &pb.Response{Message: "OK"}, nil
}

func (s *userHandlingServer) ListUsers(stream pb.UserHandling_ListUsersServer) error {

}

