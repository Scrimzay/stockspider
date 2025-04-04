package stat

import (
	"fmt"
	"github.com/Scrimzay/stockspider/event"
	"time"

	"github.com/anthdm/hollywood/actor"
)

type Stat struct {
	pair event.Pair
}

func New(pair event.Pair) actor.Producer {
	return func () actor.Receiver {
		return &Stat{
			pair: pair,
		}
	}
}

func (s *Stat) Receive(c *actor.Context) {
	switch v := c.Message().(type) {
	case actor.Started:
		fmt.Printf("Stat started: %v\n", s.pair)
	case event.Stat:
		fmt.Printf("Stat: %v\n", v)
		time.Sleep(2 * time.Second)
	}
}