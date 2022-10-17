CREATE TABLE IF NOT EXISTS queue (
    level String,
    time UInt64,
    message String
) ENGINE = Kafka SETTINGS
            kafka_broker_list = 'kafka-1:9092,kafka-2:9092',
            kafka_topic_list = 'logs',
            kafka_group_name = 'logconsumers',
            kafka_format = 'JSONEachRow',
            kafka_num_consumers = 2;

CREATE TABLE IF NOT EXISTS logs (
    level String,
    day Date,
    message String
) ENGINE = MergeTree()
ORDER BY day;

CREATE MATERIALIZED VIEW IF NOT EXISTS consumer to logs
AS SELECT level, toDateTime(time) AS day, message
FROM queue;
