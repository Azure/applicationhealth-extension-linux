package main

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Only call from an isolated namespace with an empty hosts file.
func delayedIPv4Resolver(t *testing.T, delay time.Duration) func() int32 {
	t.Helper()
	server, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var queries int32
	var workers sync.WaitGroup
	stop := make(chan struct{})
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		buffer := make([]byte, 4096)
		for {
			n, peer, err := server.ReadFrom(buffer)
			if err != nil {
				select {
				case <-stop:
					return
				default:
					t.Errorf("test DNS read failed: %v", err)
					return
				}
			}
			query := append([]byte(nil), buffer[:n]...)
			workers.Add(1)
			go func() {
				defer workers.Done()
				response, ipv4 := ipv4DNSResponse(query)
				if response == nil {
					t.Error("malformed test DNS query")
					return
				}
				if ipv4 {
					atomic.AddInt32(&queries, 1)
				}
				timer := time.NewTimer(delay)
				defer timer.Stop()
				select {
				case <-stop:
					return
				case <-timer.C:
				}
				if _, err := server.WriteTo(response, peer); err != nil {
					select {
					case <-stop:
					default:
						t.Errorf("test DNS response failed: %v", err)
					}
				}
			}()
		}
	}()
	previous := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp4", server.LocalAddr().String())
		},
	}
	t.Cleanup(func() {
		net.DefaultResolver = previous
		close(stop)
		server.Close()
		<-exited
		workers.Wait()
	})
	return func() int32 { return atomic.LoadInt32(&queries) }
}

func ipv4DNSResponse(query []byte) ([]byte, bool) {
	if len(query) < 12 || binary.BigEndian.Uint16(query[4:6]) != 1 {
		return nil, false
	}
	offset := 12
	for {
		if offset >= len(query) || query[offset] > 63 {
			return nil, false
		}
		length := int(query[offset])
		offset++
		if length == 0 {
			break
		}
		offset += length
	}
	if offset+4 > len(query) {
		return nil, false
	}
	ipv4 := binary.BigEndian.Uint16(query[offset:offset+2]) == 1
	response := append([]byte(nil), query[:offset+4]...)
	binary.BigEndian.PutUint16(response[2:4], 0x8180)
	for i := 6; i < 12; i++ {
		response[i] = 0
	}
	if ipv4 {
		binary.BigEndian.PutUint16(response[6:8], 1)
		// Compressed question name; A/IN, TTL zero, 127.0.0.1.
		response = append(response, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 0, 0, 4, 127, 0, 0, 1)
	}
	return response, ipv4
}
