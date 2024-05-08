package bitcoin

import (
	"testing"

	"github.com/ordishs/go-bitcoin"
)

func Test_Relayer(t *testing.T) {

	zmq := bitcoin.NewZMQ("signet", 18330)

	ch := make(chan []string)

	go func() {
		for c := range ch {
			t.Logf("%v", c)
		}
	}()

	// if err := zmq.Subscribe("rawblock", ch); err != nil {
	// 	t.Fatalf("%v", err)
	// }

	if err := zmq.Subscribe("hashblock", ch); err != nil {
		t.Fatalf("%v", err)
	}

	t.Log("Waiting for blocks...")

	waitCh := make(chan bool)
	<-waitCh

}
