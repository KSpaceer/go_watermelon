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

func (s *userHandlingServer) AddUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
    if ok, err := s.CheckNicknameInDatabase(ctx, user.Nickname); err != nil {
        return nil, err
    } else if ok {
        return &pb.Response{Message: "User with this nickname is already exists."}, nil
    }
    key, err := s.SetOperation(ctx, data.User{user.Nickname, user.Email}, "ADD")
    if err != nil {
        return nil, err
    }
    SendEmail(user.Email, "add")
    return &pb.Response{Message: "OK"}, nil
}

func (s *userHandlingServer) DeleteUser(ctx context.Context, user *pb.User) (*pb.Response, error) {
    if ok, err := CheckNicknameInDatabase(ctx, user.Nickname); err != nil {
        return nil, err
    } else if !ok {
        return &pb.Response{Message: "There is no user with such nickname."}, nil
    }
    SendEmail(user.Email, "delete")
    return &pb.Response{Message: "OK"}, nil
}

func (s *userHandlingServer) ListUsers(stream pb.UserHandling_ListUsersServer) error {
    dataUsersList, err := s.GetUsersFromDatabase()
    if err != nil {
        return err
    }
    pbUsersList := convertDataUsersToPBUsers(dataUsersList)
    for _, pbUser := range pbUsersList {
        if err := stream.Send(pbUser); err != nil {
            return err
        }
    }
    return nil
}

func convertDataUsersToPBUsers(dataUsers []data.User) []*pb.User {
    pbUsers := make([]*pb.User, len(dataUsers))
    for i, dataUser := range dataUsers {
        pbUsers[i] = &pb.User{Nickname: dataUser.Nickname, Email: dataUser.Email}
    }
    return psUsers
}

