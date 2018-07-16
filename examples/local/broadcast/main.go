package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"time"

	"gitlab.com/tblyler/discovery/local"
)

func main() {
	b, err := local.NewBroadcast(
		&net.UDPAddr{
			IP:   net.IPv4zero,
			Port: 50000,
		},
		nil,
	)

	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to create new broadcast:", err)
		os.Exit(1)
	}

	msgChan := make(chan []byte, 5)
	errChan := make(chan error, 2)

	ctx, cancel := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()
		errChan <- b.Listen(ctx, msgChan)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			select {
			case <-ctx.Done():
				return

			case msg := <-msgChan:
				fmt.Println("Got message:", string(msg))
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Second * time.Duration(5))
		defer ticker.Stop()

		hostname, _ := os.Hostname()
		msg := []byte(hostname)

		interfaces, _ := net.InterfaceAddrs()
		for _, inter := range interfaces {
			msg = append(msg, []byte("\n"+inter.String())...)
		}

		msg = append(msg, []byte("\n")...)

		for {
			select {
			case <-ctx.Done():
				return

			case tTime := <-ticker.C:
				fmt.Println("Sending message at", tTime)

				err := b.Send(msg)
				if err != nil {
					errChan <- err
					return
				}
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		sigChan := make(chan os.Signal)
		signal.Notify(sigChan, os.Interrupt)

		for {
			select {
			case err := <-errChan:
				fmt.Println("Got error:", err)
				return

			case <-sigChan:
				fmt.Println("Got signal to quit, quitting")
				return
			}
		}
	}()

	wg.Wait()
}
