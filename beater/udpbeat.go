package beater

import (
	"net"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/pquerna/ffjson/ffjson"
)

type UdpBeat struct {
	done   chan struct{}
	conn   *net.UDPConn
	client beat.Client
}

func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4567")
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}

	logp.Info("Receive Log on udp:%s", addr)

	client, err := b.Publisher.Connect()
	if err != nil {
		return nil, err
	}

	bt := &UdpBeat{
		done:   make(chan struct{}),
		conn:   conn,
		client: client,
	}

	return bt, nil
}

func (bt *UdpBeat) Run(b *beat.Beat) error {
	logp.Info("UdpBeat is running! Hit CTRL-C to stop it")

	udpBuf := make([]byte, 4096)

	for {
		select {
		case <-bt.done:
			return nil
		default:
		}

		// err = conn.SetDeadline()
		dataSize, err := bt.conn.Read(udpBuf)
		if err != nil {
			logp.Err("read from udp socket error")
			return err
		}

		event := beat.Event{
			Timestamp: time.Now(),
		}

		// event := common.MapStr{}
		err = ffjson.Unmarshal(udpBuf[:dataSize], &event.Fields)
		if err != nil {
			logp.Err("Counld not parse json message: %v", err)
			continue
		}

		bt.client.Publish(event)
	}
	return nil
}

func (bt *UdpBeat) Stop() {
	bt.conn.Close()
	close(bt.done)
}
