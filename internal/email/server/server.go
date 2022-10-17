package email_server

import (
	"context"
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
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
	// watermelonImgMailName defines the name of attached file.
	watermelonImgMailName = "watermelon"
	// dailyDeliveryMethodName represents a key to msgTemplates for value of daily message.
	dailyDeliveryMethodName = "sendWatermelon"
	// *SubjectName consts define subject name for different types of messages
	authMsgSubjectName  = "Confirm action"
	dailyMsgSubjectName = "Daily watermelon"
	// emailInfoFieldAmount is used to count necessary fields of SMTPServer
	emailInfoFieldAmount = 4
	// hostIP is the name of environment variable with external IP of host machine.
	hostIP = "GWM_HOST_EXTERNAL_IP"
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

	// connLimiter is a buffered channel used to limit a number of active connections.
	connLimiter chan struct{}

	// mainServiceLocation defines a location of UserHandling service (i.e. HTTP proxy) to put
	// it in templates.
	mainServiceLocation string

	// imageDirectory contains path to the directory with images.
	imageDirectory string
}

// NewEmailServer creates a new EmailServer instance using a file to configurate the SMTP Server,
// path to the main service and broker addresses to create a consumer group and log producer.
func NewEmailServer(emailInfoFilePath, mainServiceLocation, imageDirectory string, cg sarama.ConsumerGroup, lp sarama.SyncProducer) (returnedS *EmailServer, returnedErr error) {
	s := &EmailServer{}
	s.SMTPServer = mail.NewSMTPClient()
	err := s.readEmailInfoFile(emailInfoFilePath)
	if err != nil {
		return nil, err
	}
	s.mainServiceLocation, err = s.defineMainServiceLocation(mainServiceLocation)
	if err != nil {
		return nil, err
	}

	if info, err := os.Stat(imageDirectory); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory.", imageDirectory)
	}

	s.Logger = zerolog.New(io.MultiWriter(os.Stderr, kafkawriter.New(lp))).With().Timestamp().Logger()

	s.ConsumerGroup = cg

	s.connLimiter = make(chan struct{}, maxConns)
	return s, nil
}

// Wait is used to lock main goroutine until all connections are closed.
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
		ip := os.Getenv(hostIP)
		if ip == "" {
			return "", fmt.Errorf("Main service is ran on the same host, but no external path provided in environment"+
				" variable %q", hostIP)
		}
		return "http://" + ip + strings.TrimPrefix(mainServiceLocation, "localhost"), nil
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
	s.SMTPServer.Encryption = mail.EncryptionSTARTTLS
	s.SMTPServer.TLSConfig = &tls.Config{InsecureSkipVerify: true}
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

// SendDailyMessage creates a new SMTP connection through which sends a daily message
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

// chooseRandomImg picks a random image from imageDirectory.
func (s *EmailServer) chooseRandomImg() (string, error) {
	images, err := os.ReadDir(s.imageDirectory)
	if err != nil {
		return "", err
	}
	selectedFile := images[rand.Intn(len(images))]
	for selectedFile.IsDir() {
		selectedFile = images[rand.Intn(len(images))]
	}
	result, err := filepath.Abs(filepath.Join(s.imageDirectory, selectedFile.Name()))
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
				s.Info().Msg("Waiting for opening a connection...")
				s.connLimiter <- struct{}{}
				s.Info().Msgf("Connecting and sending an auth message with method %q to email %q", authInfo[2], authInfo[0])
				err := s.SendAuthMessage(authInfo[0], authInfo[1], authInfo[2])
				if err != nil {
					s.Error().Msgf("Occured while sending an auth message: %v", err)
				} else {
					s.Info().Msg("Successfully sent an email message.")
				}
				<-s.connLimiter
			}()
		case sc.DailyDeliveryTopic:
			userInfo := strings.Split(string(message.Value), " ")
			go func() {
				s.Info().Msg("Waiting for opening a connection...")
				s.connLimiter <- struct{}{}
				s.Info().Msgf("Connecting and sending a daily message to email %q", userInfo[0])
				err := s.SendDailyMessage(userInfo[0], userInfo[1])
				if err != nil {
					s.Error().Msgf("Occured while sending a daily message: %v", err)
				} else {
					s.Info().Msg("Successfully sent an email message.")
				}
				<-s.connLimiter
			}()
		}
		session.MarkMessage(message, "")
	}
	return nil
}
