package emt

import (
	"fmt"
	"github.com/galaxy-book/common/core/config"
	emitter "github.com/emitter-io/go/v2"
	"github.com/stretchr/testify/assert"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestConnect(t *testing.T) {

	config.LoadUnitTestConfig()

	client, err := GetClient()
	assert.Equal(t, err, nil)
	if err == nil{
		assert.Equal(t, client.IsConnected(), true)
	}

	for ;;{
		time.Sleep(1 * time.Second)

		client, err = GetClient()
		if err != nil{
			fmt.Println("err: ", err)
			continue
		}
		fmt.Println("is connected", client.IsConnected())
	}

}

func TestGenerateKey(t *testing.T) {
	config.LoadUnitTestConfig()

	key, err := GenerateKey("nico/#/", "rw", 10000)
	assert.Equal(t, err, nil)
	assert.NotEqual(t, key, "")
	t.Log(key)
}

func TestPublish(t *testing.T){
	config.LoadUnitTestConfig()

	channel := "nico/hello/"

	key, err := GenerateKey(channel, "rw", 10000)
	assert.Equal(t, err, nil)
	assert.NotEqual(t, key, "")
	t.Log(key)

	client, err := GetClient()
	assert.Equal(t, err, nil)
	assert.Equal(t, client.IsConnected(), true)

	client.Subscribe(key, channel, func(c *emitter.Client, m emitter.Message){
		fmt.Printf("消费到的 %s\n", string(m.Payload()))
	})

	counter := int32(0)
	for i := 0; i < 10; i ++{
		index := i
		go func() {
			for ;;{
				client, _ = GetClient()
				atomic.AddInt32(&counter, 1)
				client.Publish(key, channel, strconv.Itoa(index) + " - " + strconv.Itoa(int(counter)))
			}
		}()
	}

	time.Sleep(20 * time.Second)
	fmt.Println(counter)

}