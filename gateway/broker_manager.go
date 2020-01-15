package gateway

import (
	"log"
	"time"

	"github.com/spec-tacles/go/broker"
)

// BrokerManager manages a broker to ensure the connection stays alive
type BrokerManager struct {
	b            broker.Broker
	log          *log.Logger
	disconnected chan struct{}
}

// NewBrokerManager makes a new broker manager
func NewBrokerManager(b broker.Broker, logger *log.Logger) *BrokerManager {
	return &BrokerManager{
		b:            b,
		log:          logger,
		disconnected: make(chan struct{}),
	}
}

// Connect connects the broker and blocks until the connection cannot be recovered
func (m *BrokerManager) Connect(url string) error {
	closes := make(chan error)
	defer close(closes)

	for {
		retryInterval := time.Second * 5
		for err := m.b.Connect(url); err != nil; err = m.b.Connect(url) {
			m.log.Printf("failed to connect to broker, retrying in %d seconds: %v\n", retryInterval/time.Second, err)
			time.Sleep(retryInterval)
			if retryInterval != 80 {
				retryInterval *= 2
			}
		}

		err := m.b.NotifyClose(closes)
		if err != nil {
			m.log.Printf("failed to listen to closes: %s\n", err)
			return err
		}

		close(m.disconnected)
		m.disconnected = nil
		err = <-closes
		m.disconnected = make(chan struct{})

		m.log.Printf("connection to broker was closed (%s): reconnecting in 5s\n", err)
		time.Sleep(5 * time.Second)
	}
}

// Publish publishes an event, waiting until the connection is opened if necessary
func (m *BrokerManager) Publish(event string, data []byte) error {
	if m.disconnected != nil {
		<-m.disconnected
	}

	return m.b.Publish(event, data)
}

// Subscribe subscribes to an event, waiting until the connection is opened if necessary
func (m *BrokerManager) Subscribe(event string) error {
	if m.disconnected != nil {
		<-m.disconnected
	}

	return m.b.Subscribe(event)
}

// Unsubscribe unsubscribes from an event, waiting until the connection is opened if necessary
func (m *BrokerManager) Unsubscribe(event string) error {
	if m.disconnected != nil {
		<-m.disconnected
	}

	return m.b.Unsubscribe(event)
}

// SetCallback sets the callback
func (m *BrokerManager) SetCallback(handler broker.EventHandler) {
	m.b.SetCallback(handler)
}

func (m *BrokerManager) NotifyClose(ch chan error) error {
	return m.b.NotifyClose(ch)
}
