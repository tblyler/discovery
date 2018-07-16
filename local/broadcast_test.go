package broadcast

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBroadcastBadListenAddr(t *testing.T) {
	assert := assert.New(t)

	badAddrs := []*net.UDPAddr{
		nil,
		&net.UDPAddr{IP: net.IPv6loopback},
		&net.UDPAddr{IP: net.IPv4zero},
	}

	expectedErrs := []error{
		ErrNotIPv4,
		ErrNotIPv4,
		ErrZeroPort,
	}

	for i, badAddr := range badAddrs {
		b, err := NewBroadcast(badAddr, nil)
		assert.Nil(b)
		assert.Equal(expectedErrs[i], err)
	}
}

func TestNewBroadcastBroadcastIPSet(t *testing.T) {
	assert := assert.New(t)

	inputs := [][]net.IP{
		nil,
		DefaultBroadcastIPs(),
		[]net.IP{
			net.IPv4bcast,
		},
		[]net.IP{
			net.IPv4bcast,
			net.IPv4allsys,
			net.IPv4allrouter,
		},
	}

	expectedOutputs := [][]net.IP{
		DefaultBroadcastIPs(),
		inputs[1],
		inputs[2],
		inputs[3],
	}

	laddr := &net.UDPAddr{IP: net.IPv4zero}
	laddr.Port = 123

	for i, input := range inputs {
		b, err := NewBroadcast(laddr, input)
		assert.NoError(err)
		assert.NotNil(b)

		assert.Equal(expectedOutputs[i], b.bcastIPs)
	}
}

func TestBroadcastListenBadAddress(t *testing.T) {
	assert := assert.New(t)

	b, err := NewBroadcast(&net.UDPAddr{IP: net.IPv4zero, Port: -1}, nil)
	assert.Error(err)
	assert.Nil(b)
}

func TestBroadcastListenContextReturn(t *testing.T) {
	assert := assert.New(t)

	laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	b, err := NewBroadcast(laddr, nil)
	assert.NoError(err)
	assert.NotNil(b)
	b.listenAddr.Port = 0
	b.ListenDeadline = time.Nanosecond

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error)
	go func() {
		errChan <- b.Listen(ctx, nil)
	}()

	cancel()
	assert.NoError(<-errChan)
}

func TestBroadcastListenNonTimeoutError(t *testing.T) {
	assert := assert.New(t)

	laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	b, err := NewBroadcast(laddr, nil)
	assert.NoError(err)
	assert.NotNil(b)
	b.listenAddr.Port = 0
	b.ListenDeadline = time.Nanosecond

	errChan := make(chan error)
	go func() {
		errChan <- b.Listen(context.Background(), nil)
	}()

	// wait for the listening connectino to be created
	for b.lconn == nil {
		time.Sleep(time.Millisecond)
	}

	b.lconn.Close()

	assert.Error(<-errChan)
}

func TestBroadcastListenTimeoutErrorThenDone(t *testing.T) {
	assert := assert.New(t)

	laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	b, err := NewBroadcast(laddr, nil)
	assert.NoError(err)
	assert.NotNil(b)
	b.listenAddr.Port = 0
	b.ListenDeadline = time.Nanosecond

	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error)
	go func() {
		errChan <- b.Listen(ctx, nil)
	}()

	time.Sleep(time.Millisecond * time.Duration(10))
	cancel()

	assert.NoError(<-errChan)
}

func TestBroadcastListen(t *testing.T) {
	assert := assert.New(t)

	laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	b, err := NewBroadcast(laddr, nil)
	assert.NoError(err)
	assert.NotNil(b)
	b.listenAddr.Port = 0
	b.ListenDeadline = time.Millisecond * time.Duration(10)
	ctx, cancel := context.WithCancel(context.Background())

	errChan := make(chan error)
	msgChan := make(chan []byte)
	go func() {
		errChan <- b.Listen(ctx, msgChan)
	}()

	// wait for the listening connectino to be created
	for b.lconn == nil {
		time.Sleep(time.Millisecond)
	}

	conn, err := net.ListenUDP("udp4", nil)
	assert.NoError(err)
	defer conn.Close()

	expectedBytes := []byte("Hello there! GENERAL KENOBI!")

	_, err = conn.WriteTo(expectedBytes, b.lconn.LocalAddr())
	assert.NoError(err)

	gotBytes := <-msgChan
	cancel()
	assert.NoError(<-errChan)
	assert.Equal(expectedBytes, gotBytes)
}

func TestBroadcastSendBadmsgSize(t *testing.T) {
	assert := assert.New(t)

	laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	b, err := NewBroadcast(laddr, nil)
	assert.NoError(err)
	assert.NotNil(b)

	badMsgs := [][]byte{
		nil,
		make([]byte, 0),
		[]byte{},
		make([]byte, MaxUDPPacketSize+1),
	}

	for _, badMsg := range badMsgs {
		assert.Equal(ErrBroadcastSendBadSize, b.Send(badMsg))
	}
}

func TestBroadcastSend(t *testing.T) {
	assert := assert.New(t)

	laddr := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
	b, err := NewBroadcast(laddr, nil)
	assert.NoError(err)
	assert.NotNil(b)

	msgChan := make(chan []byte)
	errChan := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		errChan <- b.Listen(ctx, msgChan)
	}()

	// wait for the listening connectino to be created
	for b.lconn == nil {
		time.Sleep(time.Millisecond)
	}

	bClient, err := NewBroadcast((b.lconn.LocalAddr()).(*net.UDPAddr), []net.IP{(b.lconn.LocalAddr()).(*net.UDPAddr).IP})
	assert.NoError(err)

	expectedBytes := []byte("hello there, GENERAL KENOBI!!!!")
	assert.NoError(bClient.Send(expectedBytes))

	assert.Equal(expectedBytes, <-msgChan)
	cancel()
}

func TestGetBroadcastIPs(t *testing.T) {
	assert := assert.New(t)

	cidrToAddr := func(cidr string) net.Addr {
		_, net, err := net.ParseCIDR(cidr)
		assert.NoError(err)

		return net
	}

	inputAddrs := []net.Addr{
		cidrToAddr("192.168.1.0/24"),
		cidrToAddr("192.168.0.0/16"),
		cidrToAddr("192.0.0.0/8"),
		cidrToAddr("192.168.1.0/4"),
		// the following should not generate results
		cidrToAddr("127.0.0.1/24"),
		&net.IPAddr{IP: net.IPv6linklocalallnodes},
		&net.IPAddr{IP: net.IPv4bcast},
	}

	expectedAddrs := []net.IP{
		net.IPv4(192, 168, 1, 255),
		net.IPv4(192, 168, 255, 255),
		net.IPv4(192, 255, 255, 255),
		net.IPv4(207, 255, 255, 255),
	}

	gotAddrs := GetBroadcastIPs(inputAddrs)
	assert.Len(gotAddrs, len(expectedAddrs))
	for i, gotAddr := range gotAddrs {
		assert.Equal(expectedAddrs[i], gotAddr)
	}
}
