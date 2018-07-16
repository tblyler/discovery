package broadcast

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// MaxUDPPacketSize is the max packet size for a broadcast message
	// 0xffff - len(IPHeader) - len(UDPHeader)
	MaxUDPPacketSize = 0xffff - 20 - 8

	// DefaultBroadcastListenDeadline is the number of seconds to spend waiting for a packet
	// before a loop and check of the context doneness
	DefaultBroadcastListenDeadline = time.Second * time.Duration(5)

	// DefaultBroadcastSendDeadline is the number of seconds to spend sending a packet before
	// giving up for a given broadcast address
	DefaultBroadcastSendDeadline = time.Second * time.Duration(5)
)

var (
	// ErrNotIPv4 if a non-IPv4 address was given when an IPv4 address was expected
	ErrNotIPv4 = errors.New("non IPv4 address given")

	// ErrZeroPort if a zero port was defined for an IPv4 address when one must be specified
	ErrZeroPort = errors.New("missing port definition in address")

	// ErrBroadcastSendBadSize if the message size of a message to send is too large
	ErrBroadcastSendBadSize = fmt.Errorf("broadcast message size must be > 0B and  <= %dB", MaxUDPPacketSize)
)

// Broadcast implements Seeker with local network broadcasting with IPv4 broadcasting
type Broadcast struct {
	listenAddr *net.UDPAddr
	bcastIPs   []net.IP
	lconn      *net.UDPConn
	lconnLock  sync.Mutex

	ListenDeadline time.Duration
	SendDeadline   time.Duration
}

// NewBroadcast creates a new instance of Broadcast
func NewBroadcast(listenAddr *net.UDPAddr, bcastIPs []net.IP) (b *Broadcast, err error) {
	err = validateBcastListenAddr(listenAddr)
	if err != nil {
		return
	}

	if bcastIPs == nil {
		bcastIPs = DefaultBroadcastIPs()
	}

	b = &Broadcast{
		listenAddr:     listenAddr,
		bcastIPs:       bcastIPs,
		ListenDeadline: DefaultBroadcastListenDeadline,
		SendDeadline:   DefaultBroadcastSendDeadline,
	}

	return
}

// Listen for incoming Send requests from other Broadcast implementations
func (b *Broadcast) Listen(ctx context.Context, msgChan chan<- []byte) error {
	b.lconnLock.Lock()
	defer b.lconnLock.Unlock()

	var err error
	b.lconn, err = net.ListenUDP("udp4", b.listenAddr)
	if err != nil {
		return err
	}
	defer b.lconn.Close()

	for {
		select {
		case <-ctx.Done():
			return nil

		default:
			// noop
		}

		packet := make([]byte, MaxUDPPacketSize)
		b.lconn.SetReadDeadline(time.Now().Add(b.ListenDeadline))
		dataLen, _, err := b.lconn.ReadFrom(packet)
		if err != nil {
			// only return err if it was not a timeout error
			if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
				return err
			}

			continue
		}

		msgChan <- packet[:dataLen]
	}
}

// Send a message to other potential Broadcast implementations
func (b *Broadcast) Send(msg []byte) error {
	msgLen := len(msg)
	if msgLen > MaxUDPPacketSize || msgLen <= 0 {
		return ErrBroadcastSendBadSize
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	for _, ip := range b.bcastIPs {
		dst := &net.UDPAddr{IP: ip, Port: b.listenAddr.Port}
		// be vigilant about short sends
		conn.SetWriteDeadline(time.Now().Add(b.SendDeadline))
		_, err := conn.WriteTo(msg, dst)
		conn.SetWriteDeadline(time.Time{})

		if err != nil {
			// only return err if it wasn't a timeout error
			if err, ok := err.(net.Error); !ok || !err.Timeout() {
				return err
			}
		}
	}

	return nil
}

// GetBroadcastIPs from the given list of addresses
func GetBroadcastIPs(addrs []net.Addr) (bcastIPs []net.IP) {
	// IPv4 is enforced for broadcast, this will store the raw IPv4 address's value
	bCastBytes := [4]byte{}

	for _, addr := range addrs {
		ipAddr, ok := addr.(*net.IPNet)

		if ok && ipAddr.IP.IsGlobalUnicast() && ipAddr.IP.To4() != nil {
			// per https://en.wikipedia.org/wiki/Broadcast_address#IP_networking
			// The broadcast address for an IPv4 host can be obtained by performing
			// a bitwise OR operation between the bit complement of the subnet mask
			// and the host's IP address. In other words, take the host's IP address,
			// and set to '1' any bit positions which hold a '0' in the subnet mask.
			// For broadcasting a packet to an entire IPv4 subnet using the private IP
			// address space 172.16.0.0/12, which has the subnet mask 255.240.0.0, the
			// broadcast address is 172.16.0.0 | 0.15.255.255 = 172.31.255.255.
			mask := ipAddr.Mask
			mask = append(make([]byte, len(mask)-4), mask...)

			for i, num := range ipAddr.IP.To4() {
				bCastBytes[i] = num | (^mask[i])
			}

			bcastIPs = append(bcastIPs, net.IPv4(bCastBytes[0], bCastBytes[1], bCastBytes[2], bCastBytes[3]))
		}
	}

	return
}

// DefaultBroadcastIPs gets all available interface IPs for doing broadcasting
func DefaultBroadcastIPs() (bcastIPs []net.IP) {
	addrs, _ := net.InterfaceAddrs()
	bcastIPs = GetBroadcastIPs(addrs)
	if bcastIPs == nil {
		bcastIPs = append(bcastIPs, net.IPv4bcast)
	}

	return
}

func validateBcastListenAddr(listenAddr *net.UDPAddr) error {
	if listenAddr == nil || listenAddr.IP.To4() == nil {
		return ErrNotIPv4
	}

	if listenAddr.Port <= 0 {
		return ErrZeroPort
	}

	return nil
}
