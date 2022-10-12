package uh_server_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/KSpaceer/go_watermelon/internal/data"
	"github.com/KSpaceer/go_watermelon/internal/kafkawriter"
	sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"
	pb "github.com/KSpaceer/go_watermelon/internal/user_handling/proto"
	uh "github.com/KSpaceer/go_watermelon/internal/user_handling/server"

	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"google.golang.org/grpc"

	"github.com/Shopify/sarama"
	saramamock "github.com/Shopify/sarama/mocks"
)

type MockData struct {
	mock.Mock
}

func (d *MockData) Disconnect() {
	return
}

func (d *MockData) GetOperation(ctx context.Context, key string) (*data.Operation, error) {
	args := d.Called(ctx, key)
	return args.Get(0).(*data.Operation), args.Error(1)
}

func (d *MockData) SetOperation(ctx context.Context, user data.User, method string) (string, error) {
	args := d.Called(ctx, user, method)
	return args.String(0), args.Error(1)
}

func (d *MockData) CheckNicknameInDatabase(ctx context.Context, nickname string) (bool, error) {
	args := d.Called(ctx, nickname)
	return args.Bool(0), args.Error(1)
}

func (d *MockData) AddUserToDatabase(ctx context.Context, user data.User) error {
	args := d.Called(ctx, user)
	return args.Error(0)
}

func (d *MockData) DeleteUserFromDatabase(ctx context.Context, user data.User) error {
	args := d.Called(ctx, user)
	return args.Error(0)
}

func (d *MockData) GetUsersFromDatabase(ctx context.Context) ([]data.User, error) {
	args := d.Called(ctx)
	return args.Get(0).([]data.User), args.Error(1)
}

func TestAuthUserAddMethod(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	testOperation := &data.Operation{data.User{"arbuz", "arbuz@gmail.com"}, "ADD"}
	testKey := &pb.Key{Key: "KEF9cGJnPB7Ghhc-vFhouCEL7pCvOz7BjZW0ebLNBOa9qkHaVwdsrByXI002DKDyxkuk1p5_rRDCHTiKrtOtq7HHiphjnFo0Aj2srl7156uxc5_fvl9YjUcpuyabUKvHptiF--LY3_oNXmnQD44A-t3PUUIbi3QePLWo1eTCLZw"}
	mockData.On("GetOperation", ctx, testKey.Key).Return(testOperation, nil)
	mockData.On("AddUserToDatabase", ctx, testOperation.User).Return(nil)
	response, err := uhServer.AuthUser(ctx, testKey)
	testResponse := pb.Response{Message: "Method ADD was executed successfully."}
	if assert.Nil(t, err) {
		mockData.AssertExpectations(t)
		assert.Equal(t, testResponse, *response)
	}
}

func TestAuthUserDeleteMethod(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	testOperation := &data.Operation{data.User{"MelonEnjoyer", "melonsarebetter@gmail.com"}, "DELETE"}
	testKey := &pb.Key{Key: "hdAp8Gj8BLBqD3L03L6fseVtzJRJdTMr16B9_C5dYPcV0mojUbU3uw7aLODP82MuSqCOpkdfGWjt_7qaNapL-MafNr-jC5LZL19XgTyzW5cSj5grG9IdyVlzfCdpHzddpfsBv-51GKKCzmTQB3d6RAt6mTJwQ_AYsgOtBUr7nrc"}
	mockData.On("GetOperation", ctx, testKey.Key).Return(testOperation, nil)
	mockData.On("DeleteUserFromDatabase", ctx, testOperation.User).Return(nil)
	response, err := uhServer.AuthUser(ctx, testKey)
	testResponse := pb.Response{Message: "Method DELETE was executed successfully."}
	if assert.Nil(t, err) {
		mockData.AssertExpectations(t)
		assert.Equal(t, testResponse, *response)
	}
}

func TestAuthUserWrongKey(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	testOperation := &data.Operation{}
	testKey := &pb.Key{Key: "wrongkey"}
	mockData.On("GetOperation", ctx, testKey.Key).Return(testOperation, nil)
	response, err := uhServer.AuthUser(ctx, testKey)
	mockData.AssertExpectations(t)
	assert.NotNil(t, err)
	assert.Nil(t, response)
}

type MockStream struct {
	grpc.ServerStream
	mock.Mock
}

func (stream *MockStream) Send(m *pb.User) error {
	args := stream.Called(m)
	return args.Error(0)
}

func TestListUsers(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	mockStream := new(MockStream)
	testUsers := []data.User{{"pupa", "buhga@example.com"}, {"lupa", "lteria@gmail.com"}}
	mockData.On("GetUsersFromDatabase", mock.Anything).Return(testUsers, nil)
	for i := 0; i < len(testUsers); i++ {
		mockStream.On("Send", mock.Anything).Return(nil)
	}
	err := uhServer.ListUsers(nil, mockStream)
	mockData.AssertExpectations(t)
	mockStream.AssertExpectations(t)
	assert.Nil(t, err)
}

func TestAddUserNotExists(t *testing.T) {
	mockProducer := saramamock.NewSyncProducer(t, sarama.NewConfig())
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, mockProducer)
	uhServer.Logger = zerolog.Nop()
	testUser := &pb.User{Nickname: "ThomasShelby", Email: "peaky_blinders@gmail.com"}
	testKey := "NTAAOcXYBLLKj+SxtQ/cuKiBcxV/cCQENqv1IzMUGQ7HTvwokQu734r9lCvHIffD6seUcARz65hN8Ij9wU1+YwHp5YtdByEBUqm/HS4o+734vJNTtVE5BIjzHP0uflvPaCqgw3me06C2FNlRNsI5d6xOSmBM7MA8tqr0Tgb+ZjA="
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	mockData.On("CheckNicknameInDatabase", ctx, testUser.Nickname).Return(false, nil)
	mockData.On("SetOperation", ctx, data.User{testUser.Nickname, testUser.Email}, "ADD").Return(testKey, nil)
	msgChecker := func(msg *sarama.ProducerMessage) error {
		var err error
		if msg.Topic != sc.AuthTopic {
			err = fmt.Errorf("Wrong topic: expected %q but got %q", sc.AuthTopic, msg.Topic)
		} else if expected := sarama.StringEncoder(strings.Join([]string{testUser.Email, testKey, "ADD"}, " ")); msg.Value != expected {
			err = fmt.Errorf("Wrong value: expected %q but got %q", expected, msg.Value)
		}
		return err
	}
	mockProducer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(saramamock.MessageChecker(msgChecker))
	testResponse := pb.Response{Message: "Auth email is sent."}
	response, err := uhServer.AddUser(ctx, testUser)
	if assert.Nil(t, err) {
		mockData.AssertExpectations(t)
		assert.Equal(t, testResponse, *response)
	}
}

func TestAddUserExists(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	testUser := &pb.User{Nickname: "ThomasShelby", Email: "iamcertainlynotimposter@gmail.com"}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	mockData.On("CheckNicknameInDatabase", ctx, testUser.Nickname).Return(true, nil)
	response, err := uhServer.AddUser(ctx, testUser)
	mockData.AssertExpectations(t)
	assert.Nil(t, response)
	assert.NotNil(t, err)
}

func TestAddUserInvalidEmail(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	testUser := &pb.User{Nickname: "Newbie", Email: "idontknowwhatemailis"}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	mockData.On("CheckNicknameInDatabase", ctx, testUser.Nickname).Return(false, nil)
	response, err := uhServer.AddUser(ctx, testUser)
	mockData.AssertExpectations(t)
	assert.Nil(t, response)
	assert.NotNil(t, err)
}

func TestDeleteUserExists(t *testing.T) {
	mockProducer := saramamock.NewSyncProducer(t, sarama.NewConfig())
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, mockProducer)
	uhServer.Logger = zerolog.Nop()
	testUser := &pb.User{Nickname: "MelonEnjoyer", Email: "melonsarebetter@gmail.com"}
	testKey := "S6FqubLd0KzKUebq9kG6t8Zv2JkKDCl43xkcDnXR68i1uFRKoWP6y6tT4DiMbVUR5qzKPHXKXA8jaZtv1O1hACtgNfd9sKP/zfum4UKMCEdiL6P+aNf7hbK78Pwi7hDx78SU8u1euxLpt/yraaYzO/2vt6QAN7+4yVja/5g3SQ0="
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	mockData.On("CheckNicknameInDatabase", ctx, testUser.Nickname).Return(true, nil)
	mockData.On("SetOperation", ctx, data.User{testUser.Nickname, testUser.Email}, "DELETE").Return(testKey, nil)
	msgChecker := func(msg *sarama.ProducerMessage) error {
		var err error
		if msg.Topic != sc.AuthTopic {
			err = fmt.Errorf("Wrong topic: expected %q but got %q", sc.AuthTopic, msg.Topic)
		} else if expected := sarama.StringEncoder(strings.Join([]string{testUser.Email, testKey, "DELETE"}, " ")); msg.Value != expected {
			err = fmt.Errorf("Wrong value: expected %q but got %q", expected, msg.Value)
		}
		return err
	}
	mockProducer.ExpectSendMessageWithMessageCheckerFunctionAndSucceed(saramamock.MessageChecker(msgChecker))
	testResponse := pb.Response{Message: "Auth email is sent."}
	response, err := uhServer.DeleteUser(ctx, testUser)
	if assert.Nil(t, err) {
		mockData.AssertExpectations(t)
		assert.Equal(t, testResponse, *response)
	}
}

func TestDeleteUserNotExists(t *testing.T) {
	mockData := new(MockData)
	uhServer := uh.NewUserHandlingServer(mockData, nil)
	uhServer.Logger = zerolog.Nop()
	testUser := &pb.User{Nickname: "AwfulWatermelon", Email: "bebe@gmail.com"}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	mockData.On("CheckNicknameInDatabase", ctx, testUser.Nickname).Return(false, nil)
	response, err := uhServer.DeleteUser(ctx, testUser)
	mockData.AssertExpectations(t)
	assert.Nil(t, response)
	assert.NotNil(t, err)
}

func TestDailyMessagesToAllUsers(t *testing.T) {
	mockData := new(MockData)
	mockProducer := saramamock.NewSyncProducer(t, sarama.NewConfig())
	uhServer := uh.NewUserHandlingServer(mockData, mockProducer)
	uhServer.Logger = zerolog.New(kafkawriter.New(mockProducer))
	testUsers := []data.User{{"pupa", "buhga@example.com"}, {"lupa", "lteria@gmail.com"}}
	mockData.On("GetUsersFromDatabase", mock.Anything).Return(testUsers, nil)
	rand.Seed(time.Now().UnixNano())
	expectedFailCount := 0
	for i := 0; i < len(testUsers); i++ {
		if rand.Intn(5) == 0 {
			mockProducer.ExpectSendMessageAndFail(fmt.Errorf("FAIL"))
			expectedFailCount++
		} else {
			mockProducer.ExpectSendMessageAndSucceed()
		}
	}
	for i := 0; i < expectedFailCount; i++ {
		mockProducer.ExpectSendMessageAndSucceed()
	}
	uhServer.SendDailyMessagesToAllUsers()
	mockData.AssertExpectations(t)
}
