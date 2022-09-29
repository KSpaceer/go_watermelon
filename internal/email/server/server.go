package email_server

import (
    "encoding/csv"
    "path/filepath"

    "github.com/xhit/go-simple-mail/v2"

    "github.com/Shopify/sarama"
)

type EmailServer struct{
    *mail.SMTPServer
    sarama.Consumer
}

func NewEmailServer(emailInfoFilePath string, brokersAddresses []string) (*EmailServer, error) {
    s := &EmailServer{}
    s.SMTPServer = mail.NewSMTPClient()
    err := readEmailInfoFile(s, emailInfoFilePath)
    if err != nil {
        return nil, err
    }
    s.Consumer, err = sarama.NewConsumer(brokersAddresses, sarama.NewConfig())
    if err != nil {
        return nil, err
    }
    return s, nil 
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
    msg.AddTo(email).SetSubject("Daily watermelons")
    msg.SetBody(mail.TextHTML, sdnwe)


