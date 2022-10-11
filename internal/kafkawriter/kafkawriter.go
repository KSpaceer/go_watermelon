package kafkawriter

import (
    sc "github.com/KSpaceer/go_watermelon/internal/shared_consts"

    "github.com/Shopify/sarama"
)

type kafkaWriter struct {
    sarama.SyncProducer
}

func New(p sarama.SyncProducer) *kafkaWriter {
    return &kafkaWriter{p}
}

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

