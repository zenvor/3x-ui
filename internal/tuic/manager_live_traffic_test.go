package tuic

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/apernet/quic-go"
	"github.com/google/uuid"
	clientquic "github.com/quic-go/quic-go"
)

type reauditCCSnapshot struct {
	conn   *quic.Conn
	chosen string
	actual string
	sender uintptr
}

// TUIC finishes fixed-size requests with STOP_SENDING(0), which may beat FIN.
// Other cancellations still indicate a failed request.
type tuicRequestStream interface {
	io.Writer
	Close() error
	Context() context.Context
}

func closeTUICRequestStream(stream tuicRequestStream) error {
	err := stream.Close()
	var reset *quic.StreamError
	if err != nil && errors.As(context.Cause(stream.Context()), &reset) && reset.Remote && reset.ErrorCode == 0 {
		return nil
	}
	var clientReset *clientquic.StreamError
	if err != nil && errors.As(context.Cause(stream.Context()), &clientReset) && clientReset.Remote && clientReset.ErrorCode == 0 {
		return nil
	}
	return err
}

func TestCloseTUICRequestStreamAfterCancellation(t *testing.T) {
	for _, library := range []string{"apernet", "quic-go"} {
		for _, tc := range []struct {
			name   string
			code   quic.StreamErrorCode
			local  bool
			wantOK bool
		}{
			{"request consumed", 0, false, true},
			{"remote rejection", 0x100, false, false},
			{"local cancellation", 0, true, false},
		} {
			t.Run(library+"/"+tc.name, func(t *testing.T) {
				cert, key := generateTestCert(t)
				pair, err := tls.X509KeyPair(cert, key)
				if err != nil {
					t.Fatal(err)
				}
				listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{pair}, NextProtos: []string{"h3"}}, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				var stream tuicRequestStream
				var cancelWrite func()
				tlsConfig := &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}}
				if library == "apernet" {
					client, err := quic.DialAddr(ctx, listener.Addr().String(), tlsConfig, nil)
					if err != nil {
						t.Fatal(err)
					}
					defer client.CloseWithError(0, "")
					s, err := client.OpenUniStreamSync(ctx)
					if err != nil {
						t.Fatal(err)
					}
					stream = s
					cancelWrite = func() { s.CancelWrite(tc.code) }
				} else {
					client, err := clientquic.DialAddr(ctx, listener.Addr().String(), tlsConfig, nil)
					if err != nil {
						t.Fatal(err)
					}
					defer client.CloseWithError(0, "")
					s, err := client.OpenUniStreamSync(ctx)
					if err != nil {
						t.Fatal(err)
					}
					stream = s
					cancelWrite = func() { s.CancelWrite(clientquic.StreamErrorCode(tc.code)) }
				}
				server, err := listener.Accept(ctx)
				if err != nil {
					t.Fatal(err)
				}
				defer server.CloseWithError(0, "")
				if _, err := stream.Write([]byte{ProtocolVersion}); err != nil {
					t.Fatal(err)
				}
				received, err := server.AcceptUniStream(ctx)
				if err != nil {
					t.Fatal(err)
				}
				var payload [1]byte
				if _, err := io.ReadFull(received, payload[:]); err != nil {
					t.Fatal(err)
				}
				if tc.local {
					cancelWrite()
				} else {
					received.CancelRead(tc.code)
				}
				select {
				case <-stream.Context().Done():
				case <-ctx.Done():
					t.Fatal("stream cancellation was not received")
				}
				if err := closeTUICRequestStream(stream); (err == nil) != tc.wantOK {
					t.Fatalf("close error = %v, want success = %t", err, tc.wantOK)
				}
			})
		}
	}
}

func reauditActualSender(conn *quic.Conn) (string, uintptr) {
	cc, unlock := lockedCongestion(conn)
	defer unlock()
	ptr := cc.Pointer()
	if cc.Type().String() == "*ackhandler.ccAdapterEx" || cc.Type().String() == "*ackhandler.ccAdapter" {
		sender := cc.Elem().FieldByName("CC").Elem()
		return sender.Type().String(), ptr
	}
	return fmt.Sprintf("%s reno=%t", cc.Type(), cc.Elem().FieldByName("reno").Bool()), ptr
}

func reauditWantedSender(controller string) string {
	if controller == "bbr" {
		return "*bbr.bbrSender"
	}
	return "*congestion.cubicSender reno=true"
}

func TestAudit3ManagerEnsureActualSendersWithPersistentTraffic(t *testing.T) {
	cert, key := generateTestCert(t)
	_, cleanup := audit3StartSocksForManager(t, "reaudit@example.test", SocksPassword(), 99115)
	defer cleanup()
	userID := uuid.MustParse("a0000000-0000-0000-0000-000000000015")
	inst := Instance{Id: 99115, Tag: "reaudit-cc", Listen: "127.0.0.1", Certificate: string(cert), PrivateKey: string(key), CongestionControl: "new_reno", AuthenticationTimeout: 3, MaxIdleTime: 30, Clients: []TuicClientSettings{{UUID: userID.String(), Password: "secret-reaudit", Email: "reaudit@example.test"}}}
	manager := &Manager{servers: map[int]*managed{}, lastStartErr: map[int]string{}}
	if err := manager.Ensure(inst); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()
	server := manager.servers[inst.Id].server
	listener := server.quicListener
	address := server.packetConn.LocalAddr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type peer struct {
		client   *quic.Conn
		tcp      *quic.Stream
		snapshot reauditCCSnapshot
		packetID uint16
	}
	var peers []*peer
	tcpEcho := func(p *peer, message []byte) {
		t.Helper()
		_ = p.tcp.SetDeadline(time.Now().Add(2 * time.Second))
		if _, err := p.tcp.Write(message); err != nil {
			t.Fatal(err)
		}
		reply := make([]byte, len(message))
		if _, err := io.ReadFull(p.tcp, reply); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reply, message) {
			t.Fatalf("TCP echo mismatch: %q", reply)
		}
	}
	udpEcho := func(p *peer, streamMode bool, message []byte) {
		t.Helper()
		p.packetID++
		assoc := uint16(100)
		if streamMode {
			assoc = 200
		}
		var frame bytes.Buffer
		if err := WritePacket(&frame, assoc, p.packetID, 1, 0, &Address{Type: AddrTypeIPv4, IP: net.ParseIP("8.8.8.8"), Port: 53}, message); err != nil {
			t.Fatal(err)
		}
		var reader io.Reader
		if streamMode {
			stream, err := p.client.OpenUniStreamSync(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := stream.Write(frame.Bytes()); err != nil {
				t.Fatal(err)
			}
			if err := closeTUICRequestStream(stream); err != nil {
				t.Fatal(err)
			}
			response, err := p.client.AcceptUniStream(ctx)
			if err != nil {
				t.Fatal(err)
			}
			reader = response
		} else {
			if err := p.client.SendDatagram(frame.Bytes()); err != nil {
				t.Fatal(err)
			}
			response, err := p.client.ReceiveDatagram(ctx)
			if err != nil {
				t.Fatal(err)
			}
			reader = bytes.NewReader(response)
		}
		_, command, err := ReadCommand(reader)
		if err != nil || command != CmdPacket {
			t.Fatalf("UDP response command=%d error=%v", command, err)
		}
		hdr, err := ReadPacketHeader(reader)
		if err != nil {
			t.Fatal(err)
		}
		payload, err := readPacketPayload(reader, hdr)
		if err != nil {
			t.Fatal(err)
		}
		if hdr.AssocID != assoc || !bytes.Equal(payload, message) {
			t.Fatalf("UDP echo mismatch association=%d payload=%q", hdr.AssocID, payload)
		}
	}
	for step, controller := range []string{"new_reno", "reno", "bbr", "BBR", "cubic", "CuBiC", "", "invalid"} {
		inst.CongestionControl = controller
		if err := manager.Ensure(inst); err != nil {
			t.Fatal(err)
		}
		normalized, _ := normalizeCongestionControl(controller)
		served := normalized
		if served == "cubic" {
			served = "new_reno"
		}
		if server.quicListener != listener || server.packetConn.LocalAddr().String() != address {
			t.Fatal("listener changed")
		}
		client, err := quic.DialAddr(ctx, address, &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"h3"}}, &quic.Config{EnableDatagrams: true, MaxIdleTimeout: 30 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		defer client.CloseWithError(0, "")
		tlsState := client.ConnectionState().TLS
		token, err := tlsState.ExportKeyingMaterial(string(userID[:]), []byte("secret-reaudit"), 32)
		if err != nil {
			t.Fatal(err)
		}
		auth, err := client.OpenUniStreamSync(ctx)
		if err != nil {
			t.Fatal(err)
		}
		authBytes := make([]byte, 50)
		authBytes[0], authBytes[1] = ProtocolVersion, CmdAuthenticate
		copy(authBytes[2:18], userID[:])
		copy(authBytes[18:], token)
		if _, err := auth.Write(authBytes); err != nil {
			t.Fatal(err)
		}
		if step == 0 {
			// Exercise the server's STOP_SENDING arriving before the client FIN.
			select {
			case <-auth.Context().Done():
			case <-ctx.Done():
				t.Fatal("server did not finish reading the authentication request")
			}
		}
		if err := closeTUICRequestStream(auth); err != nil {
			t.Fatal(err)
		}

		waitForClientCongestionSender(t, server, client, served)
		var serverConn *quic.Conn
		server.connectionsMu.Lock()
		for candidate := range server.connections {
			if matchesClientSocket(candidate, client) {
				serverConn = candidate
				break
			}
		}
		server.connectionsMu.Unlock()
		if serverConn == nil {
			t.Fatal("server connection missing")
		}
		actual, sender := reauditActualSender(serverConn)
		snap := reauditCCSnapshot{conn: serverConn, chosen: normalized, actual: actual, sender: sender}
		if actual != reauditWantedSender(normalized) {
			t.Fatalf("wrong sender: %s", actual)
		}
		tcp, err := client.OpenStreamSync(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var connect bytes.Buffer
		connect.Write([]byte{ProtocolVersion, CmdConnect})
		if err := WriteAddress(&connect, &Address{Type: AddrTypeIPv4, IP: net.ParseIP("1.1.1.1"), Port: 80}); err != nil {
			t.Fatal(err)
		}
		if _, err := tcp.Write(connect.Bytes()); err != nil {
			t.Fatal(err)
		}
		for _, p := range peers {
			if p.snapshot.sender == snap.sender {
				t.Fatal("sender reused across connections")
			}
		}
		peers = append(peers, &peer{client: client, tcp: tcp, snapshot: snap})
		for i, p := range peers {
			actual, ptr := reauditActualSender(p.snapshot.conn)
			if actual != p.snapshot.actual || ptr != p.snapshot.sender {
				t.Fatalf("existing connection sender changed: %s -> %s", p.snapshot.actual, actual)
			}
			msg := fmt.Appendf(nil, "live-step-%d-peer-%d", step, i)
			tcpEcho(p, msg)
			udpEcho(p, false, msg)
			udpEcho(p, true, msg)
		}
		t.Logf("step=%d new=%s old peers=%d usable TCP/native UDP/stream UDP; listener preserved", step, snap.actual, len(peers)-1)
	}
}

func audit3StartSocksForManager(t *testing.T, expectedUser, expectedPass string, inboundID int) (string, func()) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", SOCKSPortForInbound(inboundID)))
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	stop := make(chan struct{})

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}
			go handleMockSocksConn(conn, expectedUser, expectedPass)
		}
	}()

	return ln.Addr().String(), func() {
		close(stop)
		_ = ln.Close()
	}
}
