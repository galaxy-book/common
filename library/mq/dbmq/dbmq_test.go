package dbmq

import (
	"fmt"
	"github.com/galaxy-book/common/core/config"
	"github.com/galaxy-book/common/core/errors"
	"github.com/galaxy-book/common/core/model"
	"strconv"
	"testing"
	"time"
)

func pushMsg(message *model.MqMessageExt) errors.SystemErrorInfo {

	if "msgeeee - 4" == message.Body {
		fmt.Printf("fail: %v \n", message)
		return errors.BuildSystemErrorInfo(errors.DbMQConsumerStartedError)
	}
	fmt.Println(message)
	return nil
}

func TestGetDbMQProxy(t *testing.T) {

	config.LoadUnitTestConfig()

	for i := 0; i < 10 ; i++ {

		msg := &model.MqMessage{

			Topic: "test_test_topic",
			Tags:  "a,b,c",
			Keys:  "123",
			Body:  "msgeeee - " + strconv.FormatInt(int64(i), 10),
		}

		msgExt, err := GetDbMQProxy().PushMessage(msg)

		fmt.Println(msgExt)
		fmt.Println(err)
	}

	go GetDbMQProxy().ConsumeMessage("test_test_topic", "consumer1", pushMsg)

	time.Sleep(time.Duration(3) * time.Second)

}
