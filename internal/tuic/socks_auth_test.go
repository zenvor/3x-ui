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

func TestSocksCredentialsByteLimits(t *testing.T) {
	for _, tc := range []struct {
		name, user, password string
		valid                bool
	}{
		{"ASCII boundary", strings.Repeat("u", 255), strings.Repeat("p", 255), true},
		{"UTF8 boundary", strings.Repeat("中", 85), "password", true},
		{"ASCII overflow", strings.Repeat("u", 256), "password", false},
		{"UTF8 overflow", strings.Repeat("中", 86), "password", false},
		{"password overflow", "user", strings.Repeat("p", 256), false},
		{"empty user", "", "password", false},
		{"empty password", "user", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := SocksInboundSettings([]string{tc.user}, tc.password)
			if (err == nil) != tc.valid {
				t.Fatalf("settings error = %v, valid = %t", err, tc.valid)
			}
			if tc.valid && len(raw) == 0 {
				t.Fatal("valid credentials produced no settings")
			}
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()
			_ = client.SetDeadline(time.Now().Add(time.Second))
			_ = server.SetDeadline(time.Now().Add(time.Second))
			result := make(chan error, 1)
			go func() {
				if tc.valid {
					result <- authenticateMockSocksConn(server, tc.user, tc.password)
				} else {
					payload, err := io.ReadAll(server)
					if err == nil && len(payload) != 0 {
						err = errors.New("invalid credentials wrote handshake bytes")
					}
					result <- err
				}
			}()
			err = socks5Handshake(client, tc.user, tc.password)
			if (err == nil) != tc.valid {
				t.Fatalf("handshake error = %v, valid = %t", err, tc.valid)
			}
			if !tc.valid && !strings.Contains(err.Error(), "1 to 255 bytes") {
				t.Fatalf("unexpected validation error: %v", err)
			}
			_ = client.Close()
			if err := <-result; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateClientsRejectsEmailByteOverflow(t *testing.T) {
	for _, email := range []string{strings.Repeat("u", 256), strings.Repeat("中", 86)} {
		if err := ValidateClients([]TuicClientSettings{{UUID: "a0000000-0000-0000-0000-000000000001", Email: email}}); err == nil {
			t.Fatal("runtime accepted an email exceeding the SOCKS username byte limit")
		}
	}
}
