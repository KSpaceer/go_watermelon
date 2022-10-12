package email_server

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xhit/go-simple-mail/v2"

	"github.com/Shopify/sarama"

	"github.com/rs/zerolog"

	"github.com/KSpaceer/go_watermelon/internal/kafkawriter"
	sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"
)

const (
	// maxConns is used to limit currently active SMTP connections.
	maxConns = 10

	// watermelonsDir contains path to the directory with images.
	watermelonsDir = "../../../img"
	// watermelonImgMailName defines the name of attached file.
	watermelonImgMailName = "watermelon"
	// dailyDeliveryMethodName represents a key to msgTemplates for value of daily message.
	dailyDeliveryMethodName = "sendWatermelon"
	// *SubjectName consts define subject name for different types of messages
	authMsgSubjectName  = "Confirm action"
	dailyMsgSubjectName = "Daily watermelon"
	// emailInfoFieldAmount is used to count necessary fields of SMTPServer
	emailInfoFieldAmount = 4
)

var (
	// msgTemplates contains HTML templates for different messages.
	msgTemplates = map[string]string{
		"ADD": `<html>
                    <head>
                        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
                        <title>Watermelon delivery</title>
                    </head>
                    <body>
                        <p>Hi! This is confirm message for subscribing to watermelon photo daily delivery service.</p>
                        <p>If you didn't try to subscribe, ignore this message.</p>
                        <p>Otherwise, <a href="%s/v1/auth/%s">click here</a></p>
                    </body>
                </html>`,
		"DELETE": `<html>
                    <head>
                        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
                        <title>Watermelon delivery</title>
                    </head>
                    <body>
                        <p>Hi! This is confirm message for unsubscribing from watermelon photo daily delivery service.</p>
                        <p>If you didn't try to unsubscribe, ignore this message.</p>
                        <p>Otherwise, <a href="%s/v1/auth/%s">click here</a></p>
                    </body>
                </html>`,
		dailyDeliveryMethodName: `<html>
                                    <head>
                                        <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
                                        <title>Here comes watermelon</title>
                                    </head>
                                    <body>
                                        <p><b>Have a nice day, %s!</b></p>
                                        <p><img src="cid:%s" alt="Watermelon" /></p>
                                    </body>
                                  </html>`}
)

// EmailServer embodies email sending service. It embeds
// SMTPServer to send email messages, Kafka ConsumerGroup to get requests
// from other services (meaning UserHandling), Logger to log events.
type EmailServer struct {
	*mail.SMTPServer
	sarama.ConsumerGroup
	zerolog.Logger

	// logProducerCloser is used to close the message broker producer used to
	// send logs to database
	logProducerCloser func() error

	// connLimiter is a buffered channel used to limit a number of active connections.
	connLimiter chan struct{}

	// mainServiceLocation defines a location of UserHandling service (i.e. HTTP proxy) to put
	// it in templates.
	mainServiceLocation string
}

// NewEmailServer creates a new EmailServer instance using a file to configurate the SMTP Server,
// path to the main service and broker addresses to create a consumer group and log producer.
func NewEmailServer(emailInfoFilePath, mainServiceLocation string, brokersAddresses []string) (*EmailServer, error) {
	s := &EmailServer{}
	s.SMTPServer = mail.NewSMTPClient()
	err := s.readEmailInfoFile(emailInfoFilePath)
	if err != nil {
		return nil, err
	}
	s.ConsumerGroup, err = sarama.NewConsumerGroup(brokersAddresses, "", sarama.NewConfig())
	if err != nil {
		return nil, err
	}
	logProducer, err := sarama.NewSyncProducer(brokersAddresses, sarama.NewConfig())
	if err != nil {
		return nil, err
	}
	s.Logger = zerolog.New(io.MultiWriter(os.Stderr, kafkawriter.New(logProducer))).With().Timestamp().Logger()
	s.logProducerCloser = logProducer.Close
	s.mainServiceLocation, err = s.defineMainServiceLocation(mainServiceLocation)
	if err != nil {
		return nil, err
	}
	s.connLimiter = make(chan struct{}, maxConns)
	return s, nil
}

// Disconnect closes all existing connectins of EmailServer.
func (s *EmailServer) Disconnect() {
	s.Info().Msg("Email server is disconnected from MB.")
	s.ConsumerGroup.Close()
	s.logProducerCloser()
}

// Wait is used to lock main goroutine until all other are closed.
func (s *EmailServer) Wait() {
	for len(s.connLimiter) != 0 {
	}
}

// SubscribeToTopics starts consuming incoming messages from other services.
func (s *EmailServer) SubscribeToTopics(ctx context.Context) error {
	for {
		if err := s.Consume(ctx, []string{sc.AuthTopic, sc.DailyDeliveryTopic}, s); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return nil
		}
	}
}

// defineMainServiceLocation replaces "localhost" with external IP. Otherwise it returns given string.
func (s *EmailServer) defineMainServiceLocation(mainServiceLocation string) (string, error) {
	if strings.HasPrefix(mainServiceLocation, "localhost") {
		interfaceAddresses, err := net.InterfaceAddrs()
		if err != nil {
			return "", err
		}
		for _, interfaceAddr := range interfaceAddresses {
			networkIP, ok := interfaceAddr.(*net.IPNet)
			if ok && !networkIP.IP.IsLoopback() && networkIP.IP.To4() != nil {
				ip := networkIP.IP.String()
				return "http://" + ip + strings.TrimPrefix(mainServiceLocation, "localhost") + "/", nil
			}
		}
		return "", fmt.Errorf("Main service is supposed to be ran on localhost, but there is no external IP.")
	} else if _, err := url.ParseRequestURI(mainServiceLocation); err != nil {
		return "", err
	}
	return mainServiceLocation, nil
}

// readEmailInfoFile reads CSV data from file (with given filepath) and
// sets SMTPServer fields.
func (s *EmailServer) readEmailInfoFile(emailInfoFilePath string) error {
	var err error
	if !filepath.IsAbs(emailInfoFilePath) {
		emailInfoFilePath, err = filepath.Abs(emailInfoFilePath)
		if err != nil {
			return err
		}
	}
	emailInfoFile, err := os.Open(emailInfoFilePath)
	if err != nil {
		return err
	}
	defer emailInfoFile.Close()
	csvReader := csv.NewReader(emailInfoFile)
	emailInfo, err := csvReader.ReadAll()
	if err != nil {
		return err
	}
	infoCount := 0
	for i := range emailInfo[0] {
		infoCount++
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
		default:
			infoCount--
		}
	}
	if infoCount != emailInfoFieldAmount {
		return fmt.Errorf("Invalid file %q: expected %d fields of info to parse, got %d.", emailInfoFilePath, emailInfoFieldAmount, infoCount)
	}
	return nil
}

// SendAuthMessage creates a new SMTP connection through which sends a new auth message
// using given email.
func (s *EmailServer) SendAuthMessage(email, key, method string) error {
	client, err := s.Connect()
	if err != nil {
		return err
	}
	defer client.Close()
	msg := mail.NewMSG()
	msg.AddTo(email).SetSubject("Confirm action")
	msgBody := s.makeAuthMessage(key, method)
	msg.SetBody(mail.TextHTML, msgBody)
	if msg.Error != nil {
		return msg.Error
	}
	err = msg.Send(client)
	if err != nil {
		return err
	}
	return nil
}

// makeAuthMessage puts given key and method into template's placeholders.
func (s *EmailServer) makeAuthMessage(key, method string) string {
	return fmt.Sprintf(msgTemplates[method], s.mainServiceLocation, key)
}

// SendDailyMessage creates a new SMTP connectin through which sends a daily message
// with random image using given email.
func (s *EmailServer) SendDailyMessage(email, nickname string) error {
	imgPath, err := s.chooseRandomImg()
	if err != nil {
		return err
	}
	client, err := s.Connect()
	if err != nil {
		return err
	}
	defer client.Close()
	msg := mail.NewMSG()
	msg.AddTo(email).SetSubject(dailyMsgSubjectName).SetListUnsubscribe(strings.Join([]string{"<", s.mainServiceLocation, "v1/unsubscribe/", nickname, ">"}, ""))
	attachedFileName := watermelonImgMailName + filepath.Ext(imgPath)
	msg.Attach(&mail.File{FilePath: imgPath, Name: attachedFileName})
	msgBody := s.makeDailyMessage(nickname, attachedFileName)
	msg.SetBody(mail.TextHTML, msgBody)
	if msg.Error != nil {
		return msg.Error
	}
	err = msg.Send(client)
	if err != nil {
		return err
	}
	return nil
}

// chooseRandomImg picks a random image from watermelonsDir.
func (s *EmailServer) chooseRandomImg() (string, error) {
	images, err := os.ReadDir(watermelonsDir)
	if err != nil {
		return "", err
	}
	selectedFile := images[rand.Intn(len(images))]
	for selectedFile.IsDir() {
		selectedFile = images[rand.Intn(len(images))]
	}
	result, err := filepath.Abs(filepath.Join(watermelonsDir, selectedFile.Name()))
	if err != nil {
		return "", nil
	}
	return result, nil
}

// makeDailyMessage puts gives nickname and filename into template's placeholders.
func (s *EmailServer) makeDailyMessage(nickname, filename string) string {
	return fmt.Sprintf(msgTemplates[dailyDeliveryMethodName], nickname, filename)
}

// Setup is defined to implement sarama.ConsumerGroupHandler
func (s *EmailServer) Setup(session sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is defined to implement sarama.ConsumerGroupHandler
func (s *EmailServer) Cleanup(session sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim is defined to implement sarama.ConsumerGroupHandler and processes incoming messages, calling
// corresponding method.
func (s *EmailServer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		switch message.Topic {
		case sc.AuthTopic:
			authInfo := strings.Split(string(message.Value), " ")
			go func() {
				s.connLimiter <- struct{}{}
				err := s.SendAuthMessage(authInfo[0], authInfo[1], authInfo[2])
				if err != nil {
					s.Error().Msgf("Occured while sending an auth message: %v", err)
				}
				<-s.connLimiter
			}()
		case sc.DailyDeliveryTopic:
			userInfo := strings.Split(string(message.Value), " ")
			go func() {
				s.connLimiter <- struct{}{}
				err := s.SendDailyMessage(userInfo[0], userInfo[1])
				if err != nil {
					s.Error().Msgf("Occured while sending a daily message: %v", err)
				}
				<-s.connLimiter
			}()
		}
		session.MarkMessage(message, "")
	}
	return nil
}
