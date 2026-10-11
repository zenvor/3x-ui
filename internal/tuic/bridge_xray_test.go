package tuic

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	_ "github.com/xtls/xray-core/app/dispatcher"
	_ "github.com/xtls/xray-core/app/proxyman/inbound"
	_ "github.com/xtls/xray-core/app/proxyman/outbound"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/infra/conf"
	_ "github.com/xtls/xray-core/proxy/freedom"
	_ "github.com/xtls/xray-core/proxy/socks"
	_ "github.com/xtls/xray-core/transport/internet/tcp"
)

func TestAuthenticatedBridgeWorksWithRealXray(t *testing.T) {
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	_ = reservation.Close()
	stream, err := BridgeStreamSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings, err := SocksInboundSettings([]string{"real-xray@example.com"}, SocksPassword())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(map[string]any{
		"inbounds":  []map[string]any{{"listen": "127.0.0.1", "port": port, "protocol": "socks", "settings": json.RawMessage(settings), "streamSettings": json.RawMessage(stream)}},
		"outbounds": []map[string]any{{"protocol": "freedom"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var config conf.Config
	if err := json.Unmarshal(raw, &config); err != nil {
		t.Fatal(err)
	}
	built, err := config.Build()
	if err != nil {
		t.Fatal(err)
	}
	server, err := core.New(built)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	identity, err := sharedBridgeIdentity()
	if err != nil {
		t.Fatal(err)
	}
	relay := &SocksRelay{Addr: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), Password: SocksPassword(), TLSConfig: identity.client.Clone()}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()
	conn, err := relay.DialTCP(ctx, "real-xray@example.com", &Address{Type: AddrTypeIPv4, IP: net.ParseIP("127.0.0.1"), Port: uint16(echo.Addr().(*net.TCPAddr).Port)})
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("verified-xray")); err != nil {
		t.Fatal(err)
	}
	data := make([]byte, len("verified-xray"))
	if _, err := io.ReadFull(conn, data); err != nil || string(data) != "verified-xray" {
		t.Fatalf("echo=%q err=%v", data, err)
	}
	_ = conn.Close()
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("TCP relay did not close")
	}
	udp, err := relay.DialUDP(ctx, "real-xray@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer udp.Close()
	udpEcho, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer udpEcho.Close()
	go func() {
		data := make([]byte, 512)
		n, peer, err := udpEcho.ReadFrom(data)
		if err == nil {
			_, _ = udpEcho.WriteTo(data[:n], peer)
		}
	}()
	_ = udp.udpConn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := udp.Send(&Address{Type: AddrTypeIPv4, IP: net.ParseIP("127.0.0.1"), Port: uint16(udpEcho.LocalAddr().(*net.UDPAddr).Port)}, []byte("verified-udp")); err != nil {
		t.Fatal(err)
	}
	_, payload, err := udp.Receive(make([]byte, 512))
	if err != nil || string(payload) != "verified-udp" {
		t.Fatalf("UDP echo=%q err=%v", payload, err)
	}
}
