package tuic

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSocksRelayRejectsAuthenticationDowngrade(t *testing.T) {
	for _, network := range []string{"TCP", "UDP"} {
		t.Run(network, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			result := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					result <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
				var greeting [3]byte
				if _, err := io.ReadFull(conn, greeting[:]); err != nil {
					result <- err
					return
				}
				if greeting != [3]byte{5, 1, 2} {
					result <- errors.New("client offered unauthenticated access")
					return
				}
				if _, err := conn.Write([]byte{5, 0}); err != nil {
					result <- err
					return
				}
				var payload [1]byte
				_, err = conn.Read(payload[:])
				if !errors.Is(err, io.EOF) {
					result <- errors.New("client sent traffic after authentication downgrade")
					return
				}
				result <- nil
			}()
			relay := &SocksRelay{Addr: listener.Addr().String(), Password: SocksPassword()}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if network == "TCP" {
				conn, dialErr := relay.DialTCP(ctx, "restricted@test", &Address{Type: AddrTypeDomain, Host: "example.test", Port: 443})
				if conn != nil {
					_ = conn.Close()
				}
				err = dialErr
			} else {
				session, dialErr := relay.DialUDP(ctx, "restricted@test")
				if session != nil {
					_ = session.Close()
				}
				err = dialErr
			}
			if err == nil || !strings.Contains(err.Error(), "unauthenticated") {
				t.Fatalf("downgrade error = %v", err)
			}
			select {
			case err := <-result:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal("relay did not close downgraded connection")
			}
		})
	}
}
