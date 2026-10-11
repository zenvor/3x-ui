package tuic

import (
	"bytes"
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"
)

func TestBridgeRejectsImpostorBeforeSocksCredentials(t *testing.T) {
	identity, err := sharedBridgeIdentity()
	if err != nil {
		t.Fatal(err)
	}
	for _, network := range []string{"TCP", "UDP"} {
		for _, impostorTLS := range []bool{false, true} {
			name := network + "/plain"
			if impostorTLS {
				name = network + "/wrong-certificate"
			}
			t.Run(name, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				if impostorTLS {
					other, err := newBridgeIdentity()
					if err != nil {
						t.Fatal(err)
					}
					listener = tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{other.certificate}, MinVersion: tls.VersionTLS13})
				}
				captured := make(chan []byte, 1)
				go func() {
					conn, err := listener.Accept()
					if err != nil {
						captured <- nil
						return
					}
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
					var greeting [3]byte
					if _, err := io.ReadFull(conn, greeting[:]); err != nil {
						captured <- nil
						return
					}
					if greeting != [3]byte{5, 1, 2} {
						_, _ = conn.Write([]byte{5, 2})
						captured <- greeting[:]
						return
					}
					// A rogue SOCKS server accepts any credentials, without
					// having to know the bridge password beforehand.
					_, _ = conn.Write([]byte{5, 2})
					var header [2]byte
					_, _ = io.ReadFull(conn, header[:])
					user := make([]byte, int(header[1])+1)
					_, _ = io.ReadFull(conn, user)
					password := make([]byte, int(user[len(user)-1]))
					_, _ = io.ReadFull(conn, password)
					captured <- append(user, password...)
					_, _ = conn.Write([]byte{1, 0})
					request := make([]byte, 512)
					_, _ = conn.Read(request)
					_, _ = conn.Write([]byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80})
				}()
				relay := &SocksRelay{Addr: listener.Addr().String(), Password: "private-bridge-secret", TLSConfig: identity.client.Clone()}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				if network == "TCP" {
					conn, err := relay.DialTCP(ctx, "victim@example.com", &Address{Type: AddrTypeDomain, Host: "secret.example.com", Port: 443})
					if conn != nil {
						_ = conn.Close()
					}
					if err == nil {
						t.Fatal("accepted impostor TCP bridge")
					}
				} else {
					session, err := relay.DialUDP(ctx, "victim@example.com")
					if session != nil {
						_ = session.Close()
					}
					if err == nil {
						t.Fatal("accepted impostor UDP bridge")
					}
				}
				select {
				case data := <-captured:
					for _, secret := range []string{"victim@example.com", relay.Password} {
						if bytes.Contains(data, []byte(secret)) {
							t.Fatalf("impostor obtained %q", secret)
						}
					}
				case <-ctx.Done():
					t.Fatal("impostor listener did not finish")
				}
			})
		}
	}
}
