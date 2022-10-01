package email_server

import (
    "encoding/csv"
    "fmt"
    "path/filepath"
    "strings"
    "math/rand"

    "github.com/xhit/go-simple-mail/v2"

    "github.com/Shopify/sarama"

    sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"
)

const (
    maxConns = 10
    watermelonsDir = "./img"    
    watermelonImgMailName = "watermelon"
    dailyDeliveryMethodName = "sendWatermelon"
)

type EmailServer struct {
    *mail.SMTPServer
    sarama.ConsumerGroup
}

func NewEmailServer(emailInfoFilePath string, brokersAddresses []string) (*EmailServer, error) {
    s := &EmailServer{}
    s.SMTPServer = mail.NewSMTPClient()
    err := readEmailInfoFile(s, emailInfoFilePath)
    if err != nil {
        return nil, err
    }
    s.ConsumerGroup, err = sarama.NewConsumerGroup(brokersAddresses, "",  sarama.NewConfig())
    if err != nil {
        return nil, err
    }
    return s, nil 
}

func (s *EmailServer) SubscribeToTopics(ctx context.Context, topics []string) error {
    return s.Consume(ctx, topics, s)
}

func readEmailInfoFile(s *EmailServer, emailInfoFilePath string) error {
    if !filepath.IsAbs(emailInfoFilePath) {
        emailInfoFilePath, err = filepath.Abs(emailInfoFilePath)
        if err != nil {
            return err
        }
    }
    emailInfoFile, err := os.Open(emailInfoFilePath)
    defer emailInfoFile.Close()
    if err != nil {
        return err
    }
    csvReader := csv.NewReader(emailInfoFile)
    emailInfo, err := csvReader.ReadAll()
    if err != nil {
        return err
    }
    for i := range emailInfo[0] {
        switch emailInfo[0][i] {
        case "Host":
            s.SMTPServer.Host = emailInfo[1][i]
        case "Port":
            s.SMTPServer.Port, err = strconv.Atoi(emailInfo[1][i])
            if err != nil {
                return err
            }
        case "Username":
            s.SMTPServer.Username = emailInfo[1][i]
        case "Password":
            s.SMTPServer.Password = emailInfo[1][i]
        }
    }
    return nil
}

func (s *EmailServer) SendAuthMessage(email, key, method string) error {
    client, err := s.Connect()
    if err != nil {
        return err
    }
    msg := mail.NewMSG()
    msg.AddTo(email).SetSubject("Confirm action")
    msgBody := makeAuthMessage(key, method)
    msg.SetBody(mail.TextHTML, msgBody)
    if email.Error != nil {
        return email.Error
    }
    err = email.Send(client)
    if err != nil {
        return err
    }
    return nil
}

func makeAuthMessage(key, method string) string {
   return fmt.Sprintf(msgTemplates[method], key) 
}

func (s *EmailServer) SendDailyMessage(email, nickname string) error {
    imgPath, err := chooseRandomImg()
    if err != nil {
        return err
    }
    client, err := s.Connect()
    if err != nil {
        return err
    }
    msg := mail.NewMSG()
    msg.AddTo(email).SetSubject("Daliy watermelon").SetListUnsubscribe() // TODO: List Unsubscribe
    attachedFileName := watermelonImgMailName + filepath.Ext(imgPath)
    msg.Attach(&mail.File{FilePath: imgPath, Name: attachedFileName})
    msgBody := makeDailyMessage(nickname, attachedFileName)
    msg.SetBody(mail.TextHTML, msgBody)
    if email.Error != nil {
        return email.Error
    }
    err = email.Send(client)
    if err != nil {
        return err
    }
    return nil
}

func chooseRandomImg() (string, error) {
    images, err := os.ReadDir(watermelonsDir)
    if err != nil {
        return "", err
    }
    selectedFile := images[rand.Intn(len(images))]
    for selectedFile.IsDir() {
        selectedFile := images[rand.Intn(len(images))]
    }
    return filepath.Join(watermelonsDir, selectedFile), nil
}

func makeDailyMessage(nickname, filename string) string {
    return fmt.Sprintf(msgTemplates[dailyDeliveryMethodName], nickname, filename)
}

func (s *EmailServer) Setup(session sarama.ConsumerGroupSession) error {
    return nil
}

func (s *EmailServer) Cleanup(session sarama.ConsumerGroupSession) error {
    return nil
}

func (s *EmailServer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
    connLimiter := make(chan struct{}, maxConns)
    for message := range claim.Messages() {
        switch message.Topic {
        case sc.AuthTopic:
            authInfo := strings.Split(string(message.Value), " ")
            go func() {
                connLimiter <- struct{}{}
                s.SendAuthMessage(authInfo[0], authInfo[1], authInfo[2]) // TODO error channel
                <-connLimiter
            }
        case sc.DailyDeliveryTopic:
            userInfo := strings.Split(string(message.Value), " ")
            go func() {
                connLimiter <- struct{}{}
                s.SendDailyMessage(userInfo[0], userInfo[1])
                <-connLimiter
            }
        }
        session.MarkMessage(message, "")
    }
    return nil
}

