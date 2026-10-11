package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xtls/xray-core/app/router"
	xraynet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/session"
	routing_session "github.com/xtls/xray-core/features/routing/session"
	"github.com/xtls/xray-core/infra/conf"
	"github.com/xtls/xray-core/proxy/socks"
	"google.golang.org/protobuf/proto"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/tuic"
	"github.com/mhsanaei/3x-ui/v3/internal/util/json_util"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

func TestTuicSocksClientIdentitiesMatchDistinctRoutingRules(t *testing.T) {
	cfg := &xray.Config{}
	injectTuicSocks(cfg, []*model.Inbound{{Id: 5, Tag: "tuic", Protocol: model.TUIC, Enable: true, Settings: `{"clients":[
		{"uuid":"a0000000-0000-0000-0000-000000000001","password":"p1","email":"restricted@test"},
		{"uuid":"a0000000-0000-0000-0000-000000000002","password":"p2","email":"allowed@test"}]}`}})
	if len(cfg.InboundConfigs) != 1 {
		t.Fatal("TUIC relay missing")
	}
	var settings conf.SocksServerConfig
	if err := json.Unmarshal(cfg.InboundConfigs[0].Settings, &settings); err != nil {
		t.Fatal(err)
	}
	built, err := settings.Build()
	if err != nil {
		t.Fatal(err)
	}
	server := built.(*socks.ServerConfig)
	if server.AuthType != socks.AuthType_PASSWORD || len(server.Accounts) != 2 {
		t.Fatal("both identities must be authenticated by Xray")
	}
	r := new(router.Router)
	rules := &router.Config{Rule: []*router.RoutingRule{
		{UserEmail: []string{"restricted@test"}, TargetTag: &router.RoutingRule_Tag{Tag: "blocked"}},
		{UserEmail: []string{"allowed@test"}, TargetTag: &router.RoutingRule_Tag{Tag: "allowed"}},
		{Networks: []xraynet.Network{xraynet.Network_TCP}, TargetTag: &router.RoutingRule_Tag{Tag: "fallback"}},
	}}
	if err := r.Init(context.Background(), rules, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	for email, want := range map[string]string{"restricted@test": "blocked", "allowed@test": "allowed"} {
		if server.Accounts[email] != tuic.SocksPassword() {
			t.Fatal("relay credential does not match manager")
		}
		ctx := session.ContextWithInbound(context.Background(), &session.Inbound{Tag: "tuic", User: &protocol.MemoryUser{Email: email, Level: server.UserLevel}})
		ctx = session.ContextWithOutbounds(ctx, []*session.Outbound{{Target: xraynet.TCPDestination(xraynet.DomainAddress("example.test"), 443)}})
		route, err := r.PickRoute(routing_session.AsRoutingContext(ctx))
		if err != nil {
			t.Fatal(err)
		}
		if route.GetOutboundTag() != want {
			t.Fatalf("client %s routed to %s, want %s", email, route.GetOutboundTag(), want)
		}
	}
}

func TestInjectTuicSocksPreservesIdentityWithoutDuplicateAccounting(t *testing.T) {
	const originalPolicy = `{"levels":{"0":{"handshake":4,"statsUserUplink":true,"statsUserDownlink":true,"statsUserOnline":true},"4294967295":{"connIdle":77}},"system":{"statsInboundUplink":true}}`
	cfg := &xray.Config{
		Policy:         json_util.RawMessage(originalPolicy),
		InboundConfigs: []xray.InboundConfig{{Tag: "existing", Protocol: "vless", Settings: json_util.RawMessage(`{"clients":[{"level":4294967294}]}`)}},
	}
	settings := `{"clients":[
		{"uuid":"a0000000-0000-0000-0000-000000000001","password":"p","email":"active@test","enable":true},
		{"uuid":"a0000000-0000-0000-0000-000000000002","password":"p","email":"disabled@test","enable":false},
		{"password":"p","email":"invalid@test"}]}`
	inbounds := []*model.Inbound{
		{Id: 5, Tag: "tuic-5", Protocol: model.TUIC, Enable: true, Settings: settings},
		{Id: 6, Tag: "tuic-6", Protocol: model.TUIC, Enable: true, Settings: settings},
	}
	injectTuicSocks(cfg, inbounds)
	if len(cfg.InboundConfigs) != 3 {
		t.Fatalf("inbound count = %d", len(cfg.InboundConfigs))
	}
	const relayLevel = ^uint32(0) - 2
	for _, inbound := range cfg.InboundConfigs[1:] {
		var parsed conf.SocksServerConfig
		if err := json.Unmarshal(inbound.Settings, &parsed); err != nil {
			t.Fatal(err)
		}
		built, err := parsed.Build()
		if err != nil {
			t.Fatal(err)
		}
		server := built.(*socks.ServerConfig)
		if server.AuthType != socks.AuthType_PASSWORD || !server.UdpEnabled || server.UserLevel != relayLevel {
			t.Fatal("Xray relay must use password authentication, UDP, and an isolated accounting policy")
		}
		if len(server.Accounts) != 1 || server.Accounts["active@test"] != tuic.SocksPassword() {
			t.Fatal("only enabled, usable TUIC identities may authenticate with the manager's password")
		}
	}
	var before, after conf.PolicyConfig
	if err := json.Unmarshal([]byte(originalPolicy), &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cfg.Policy, &after); err != nil {
		t.Fatal(err)
	}
	beforeBuilt, err := before.Build()
	if err != nil {
		t.Fatal(err)
	}
	afterBuilt, err := after.Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []uint32{0, ^uint32(0)} {
		if !proto.Equal(beforeBuilt.Level[level], afterBuilt.Level[level]) {
			t.Fatalf("existing policy level %d changed", level)
		}
	}
	if !proto.Equal(beforeBuilt.System, afterBuilt.System) {
		t.Fatal("inbound accounting policy changed")
	}
	relay := afterBuilt.Level[relayLevel]
	if relay == nil || relay.Stats.UserUplink || relay.Stats.UserDownlink || relay.Stats.UserOnline {
		t.Fatal("the relay must not add Xray user bytes or loopback online identities on top of TUIC's own accounting")
	}
	if relay.Timeout.Handshake.GetValue() != 4 {
		t.Fatal("relay connection limits must inherit level 0")
	}
	policy := string(cfg.Policy)
	injectTuicSocks(cfg, inbounds)
	if len(cfg.InboundConfigs) != 3 || string(cfg.Policy) != policy {
		t.Fatal("duplicate injection changed relay accounts or policy")
	}
}

func TestInjectTuicSocksRejectsUnusableRelays(t *testing.T) {
	for _, tc := range []struct {
		name             string
		settings         string
		disabled, remote bool
		policy           string
	}{
		{name: "disabled", settings: `{"clients":[{"uuid":"u","password":"p","email":"e"}]}`, disabled: true},
		{name: "remote", settings: `{"clients":[{"uuid":"u","password":"p","email":"e"}]}`, remote: true},
		{name: "malformed settings", settings: `{`},
		{name: "no active clients", settings: `{"clients":[{"uuid":"u","password":"p","email":"e","enable":false}]}`},
		{name: "no identity", settings: `{"clients":[{"uuid":"u","password":"p"}]}`},
		{name: "invalid policy", settings: `{"clients":[{"uuid":"u","password":"p","email":"e"}]}`, policy: `{"levels":42}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inbound := &model.Inbound{Id: 5, Tag: "tuic", Protocol: model.TUIC, Enable: !tc.disabled, Settings: tc.settings}
			if tc.remote {
				id := 1
				inbound.NodeID = &id
			}
			cfg := &xray.Config{Policy: json_util.RawMessage(tc.policy)}
			injectTuicSocks(cfg, []*model.Inbound{inbound})
			if len(cfg.InboundConfigs) != 0 || string(cfg.Policy) != tc.policy {
				t.Fatal("unusable relay must not expose a listener or change policy")
			}
		})
	}
}
