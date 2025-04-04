package symbol

import (
	"fmt"
	"github.com/Scrimzay/stockspider/actor/stat"
	"github.com/Scrimzay/stockspider/event"
	"time"

	"github.com/anthdm/hollywood/actor"
)

type Symbol struct {
	pair event.Pair
	statPID *actor.PID
}

func New(pair event.Pair) actor.Producer {
	return func () actor.Receiver {
		return &Symbol{
			pair: pair,
		}
	}
}

func (s *Symbol) Receive(c *actor.Context) {
	switch v := c.Message().(type) {
	case actor.Started:
		s.start(c)
	case event.StockTrade:
		fmt.Printf("Trade: %v\n", v)
		time.Sleep(2 * time.Second)
	case event.Stat:
		c.Forward(s.statPID)
	}
}

func (s *Symbol) start(c *actor.Context) {
	s.statPID = c.SpawnChild(stat.New(s.pair), "stat")
}