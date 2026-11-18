package service

import (
	"testing"

	xdatabase "github.com/mhsanaei/3x-ui/v3/internal/database"
	xmodel "github.com/mhsanaei/3x-ui/v3/internal/database/model"
	submodel "github.com/mhsanaei/3x-ui/v3/subconverter/model"
)

// Settings JSON can keep an old UUID after an identity change (#6436); Xray runs
// with the clients table, so a feed built from the JSON fails to authenticate.
func TestResolveUsesClientsTableUUIDWhenSettingsStale(t *testing.T) {
	setupTestDB(t)
	db := xdatabase.GetDB()
	const stale = "11111111-1111-1111-1111-111111111111"
	const fresh = "22222222-2222-2222-2222-222222222222"
	inbound := vlessInboundWithClients(20, []xmodel.Client{testClient(stale, "bob@x")}, realityStream())
	if err := db.Create(inbound).Error; err != nil {
		t.Fatalf("seed inbound: %v", err)
	}
	record := &xmodel.ClientRecord{Email: "bob@x", UUID: fresh, Enable: true}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("seed client record: %v", err)
	}
	if err := db.Create(&xmodel.ClientInbound{ClientId: record.Id, InboundId: inbound.Id}).Error; err != nil {
		t.Fatalf("link client: %v", err)
	}

	sources, err := NewInboundResolver().Resolve([]submodel.SubscriptionInbound{{InboundId: 20, ClientEmail: "bob@x"}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(sources) != 1 {
		t.Fatalf("sources = %d, want 1", len(sources))
	}
	if got := sources[0].Client.ID; got != fresh {
		t.Fatalf("client UUID = %q, want clients-table UUID %q", got, fresh)
	}
}
