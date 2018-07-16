package discovery

import "context"

// Seeker defines a way to send/recv messages to potential peers
type Seeker interface {
	Listen(ctx context.Context, msgChan chan<- []byte) error
	Send(msg []byte) error
}
