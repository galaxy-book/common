package kafka

import (
	"gitea.bjx.cloud/allstar/common/core/config"
	"gitea.bjx.cloud/allstar/common/core/errors"
	"gitea.bjx.cloud/allstar/common/core/logger"
	"gitea.bjx.cloud/allstar/common/core/model"
	"gitea.bjx.cloud/allstar/common/core/util/json"
	"gitea.bjx.cloud/allstar/common/core/util/strs"
	"gitea.bjx.cloud/allstar/common/core/util/uuid"
	"gitea.bjx.cloud/allstar/common/library/cache"
	"github.com/Shopify/sarama"
	"strconv"
	"strings"
	"time"
)

type Proxy struct {
	//key: topic + partitioner
	producers map[string]sarama.AsyncProducer
}

var (
	log     = logger.GetMQLogger()
	version = sarama.V2_3_0_0

	producerConfig = sarama.NewConfig()
)

const (
	RecordHeaderReconsumeTimes = "ReconsumeTimes"
	RecordHeaderRePushTimes    = "RePushTimes"
)

func init() {
	//生产者通用配置
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producerConfig.Producer.Partitioner = sarama.NewRandomPartitioner
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.Return.Errors = true
	producerConfig.Version = version
}

type MsgMetadata struct {
	//消费重试次数
	ReconsumeTimes int
	//推送重试次数
	RePushTimes int
}

func getKafkaConfig() *config.KafkaMQConfig {
	return config.GetKafkaConfig()
}

func (proxy *Proxy) getProducerAutoConnect(topic string, partition int32) (*sarama.AsyncProducer, errors.SystemErrorInfo) {
	//key := topic + "#" + strconv.Itoa(int(partition))
	producer, err := proxy.getProducer(topic, partition)
	if err != nil {
		log.Error(strs.ObjectToString(err))
		return nil, err
	}
	p := *producer

	p.Errors()

	//TODO(nico) 这里做一下断开重连的逻辑
	return producer, nil
}

func (proxy *Proxy) getProducer(topic string, partition int32) (*sarama.AsyncProducer, errors.SystemErrorInfo) {
	key := topic + "#" + strconv.Itoa(int(partition))
	if proxy.producers == nil {
		proxy.producers = map[string]sarama.AsyncProducer{}
	}

	if v, ok := proxy.producers[key]; ok {
		return &v, nil
	}

	uuid := uuid.NewUuid()
	suc, err := cache.TryGetDistributedLock(key, uuid)
	if err != nil {
		log.Error(strs.ObjectToString(err))
		return nil, errors.BuildSystemErrorInfo(errors.GetDistributedLockError, err)
	}
	if suc {
		//如果获取到锁，则开始初始化
		defer cache.ReleaseDistributedLock(key, uuid)
	}

	//二次确认
	if v, ok := proxy.producers[key]; ok {
		return &v, nil
	}

	//重新构造producer
	producer, err1 := proxy.buildProducer()
	if err1 != nil {
		log.Error(strs.ObjectToString(err1))
		return nil, err1
	}

	proxy.producers[key] = *producer
	return producer, nil
}

func (proxy *Proxy) buildProducer() (*sarama.AsyncProducer, errors.SystemErrorInfo) {
	kafkaConfig := getKafkaConfig()
	log.Infof("build producer")

	producer, err := sarama.NewAsyncProducer(strings.Split(kafkaConfig.NameServers, ","), producerConfig)
	if err != nil {
		log.Infof("producer_test create producer error :%#v", err)
		return nil, errors.BuildSystemErrorInfo(errors.KafkaMqSendMsgError, err)
	}
	return &producer, nil
}

func (proxy *Proxy) PushMessage(messages ...*model.MqMessage) (*[]model.MqMessageExt, errors.SystemErrorInfo) {
	if messages == nil || len(messages) == 0 {
		return nil, errors.BuildSystemErrorInfo(errors.KafkaMqSendMsgCantBeNullError)
	}

	msgExts := make([]model.MqMessageExt, len(messages))
	for i, message := range messages {
		//传递metadata，方便消费端重试
		ReconsumeTimes := config.GetKafkaConfig().ReconsumeTimes
		RePushTimes := config.GetKafkaConfig().RePushTimes
		if message.ReconsumeTimes != nil {
			ReconsumeTimes = *message.ReconsumeTimes
		}
		if message.RePushTimes != nil {
			RePushTimes = *message.RePushTimes
		}

		key := uuid.NewUuid()
		// send message
		msg := &sarama.ProducerMessage{
			Topic:     message.Topic,
			Partition: message.Partition,
			Key:       sarama.StringEncoder(key),
			Value:     sarama.ByteEncoder(message.Body),
			Headers: []sarama.RecordHeader{
				{
					Key:   []byte(RecordHeaderReconsumeTimes),
					Value: []byte(strconv.Itoa(ReconsumeTimes)),
				},
			},
		}
		p, err1 := proxy.getProducerAutoConnect(message.Topic, message.Partition)
		if err1 != nil {
			log.Error(strs.ObjectToString(err1))
			return nil, err1
		}
		producer := *p

		var pushErr error = nil
		for rePushTime := int(0); rePushTime <= RePushTimes; rePushTime++ {
			if rePushTime > 0 {
				log.Infof("重试次数%d，最大次数%d, 上次失败原因%v, 消息内容%s", rePushTime, message.RePushTimes, pushErr, json.ToJsonIgnoreError(message))
			}
			producer.Input() <- msg
			select {
			case suc := <-producer.Successes():
				log.Infof("推送成功, offset: %d,  timestamp: %s， 消息内容%s", suc.Offset, suc.Timestamp.String(), json.ToJsonIgnoreError(message))
				pushErr = nil
			case fail := <-producer.Errors():
				log.Errorf("err: %s\n", fail.Err.Error())
				pushErr = fail
				//return nil, errors.BuildSystemErrorInfo(errors.KafkaMqSendMsgError, fail)
				time.Sleep(time.Duration(5) * time.Second)
			}
			if pushErr == nil {
				break
			}
		}
		if pushErr != nil {
			//最终推送失败，记log
			log.Errorf("消息推送失败，无重试次数，消息内容：%s", json.ToJsonIgnoreError(message))
			return nil, errors.BuildSystemErrorInfo(errors.KafkaMqSendMsgError, pushErr)
		}

		msgExts[i] = model.MqMessageExt{
			MqMessage: model.MqMessage{
				Topic:     msg.Topic,
				Body:      message.Body,
				Keys:      key,
				Partition: msg.Partition,
				Offset:    msg.Offset,
			},
		}
		log.Infof("消息发送成功 %s", json.ToJsonIgnoreError(msgExts))
	}
	return &msgExts, nil
}
