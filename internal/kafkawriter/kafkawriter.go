package kafkawriter

import (
    sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"

    "github.com/Shopify/sarama"
)

// kafkaWriter implements io.Writer interface and
// is used to send log messages to the Kafka
type kafkaWriter struct {
    sarama.SyncProducer
}

// New returns a new instance of kafkaWriter
// which will send messages using SyncProducer p
func New(p sarama.SyncProducer) *kafkaWriter {
    return &kafkaWriter{p}
}

// Write uses p as a value of a new producer message. It allows
// kafkaWriter to implement io.Writer interface
func (kw *kafkaWriter) Write(p []byte) (n int, err error) {
    msg := &sarama.ProducerMessage{
        Topic: sc.LogsTopic,
        Value: sarama.ByteEncoder(p),
        }
    _, _, err = kw.SendMessage(msg)
    if err != nil {
        return 0, err
    }
    return len(p), nil
}

