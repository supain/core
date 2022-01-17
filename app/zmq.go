package app

import (
	"fmt"

	zmq "github.com/pebbe/zmq4"
)

const zmqAddress = "ipc:///dev/shm/terracore"

type ZmqMessage struct {
	AssetName string
	AssetIn   string
	Amount    string
	Hash      string
}

func NewPubZmq() *zmq.Socket {
	publisher, err := zmq.NewSocket(zmq.PUB)
	if err != nil {
		fmt.Print("zmq error")
	}
	publisher.Bind(zmqAddress)
	return publisher
}

func (app *TerraApp) ZmqSendMessage(key string, value []byte) {
	app.pubZmq.Send(key, zmq.SNDMORE)
	app.pubZmq.SendBytes(value, 0)
}
