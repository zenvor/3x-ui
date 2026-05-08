package service

import (
	"fmt"
	"testing"

	"github.com/mhsanaei/3x-ui/v2/subconverter/database"
	"github.com/mhsanaei/3x-ui/v2/subconverter/model"
)

// seedSubscription writes a subscription row directly so the IP service has a
// foreign-key target without going through SubscriptionService.Create (which
// would require an inbounds list and obscure these focused tests).
func seedSubscription(t *testing.T, maxIps int) *model.Subscription {
	t.Helper()
	sub := &model.Subscription{
		Token:   "tk-" + t.Name(),
		Remark:  "test",
		MaxIps:  maxIps,
		Enabled: true,
	}
	if err := database.GetDB().Create(sub).Error; err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return sub
}

func TestEnforceBindsNewIP(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 2)
	svc := NewIPBindingService()

	if err := svc.Enforce(sub.Id, sub.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("first IP should bind: %v", err)
	}

	var count int64
	database.GetDB().Model(&model.IpBinding{}).Where("subscription_id = ?", sub.Id).Count(&count)
	if count != 1 {
		t.Fatalf("binding count = %d, want 1", count)
	}
}

func TestEnforceRefreshesExistingIP(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 2)
	svc := NewIPBindingService()

	if err := svc.Enforce(sub.Id, sub.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := svc.Enforce(sub.Id, sub.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("second call (same IP): %v", err)
	}

	var count int64
	database.GetDB().Model(&model.IpBinding{}).Where("subscription_id = ?", sub.Id).Count(&count)
	if count != 1 {
		t.Fatalf("re-bind should not duplicate row, got count = %d", count)
	}
}

func TestEnforceRejectsBeyondQuota(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 2)
	svc := NewIPBindingService()

	for _, ip := range []string{"1.1.1.1", "2.2.2.2"} {
		if err := svc.Enforce(sub.Id, sub.MaxIps, ip); err != nil {
			t.Fatalf("ip %s should fit within quota: %v", ip, err)
		}
	}
	if err := svc.Enforce(sub.Id, sub.MaxIps, "3.3.3.3"); err != ErrIPLimitExceeded {
		t.Fatalf("3rd IP err = %v, want ErrIPLimitExceeded", err)
	}
}

func TestEnforceUnlimited(t *testing.T) {
	setupTestDB(t)
	// MaxIps is passed explicitly as 0 below; the row's stored default does
	// not matter because Enforce trusts its argument.
	sub := seedSubscription(t, 1)
	svc := NewIPBindingService()

	for i := 1; i <= 10; i++ {
		ip := fmt.Sprintf("10.0.0.%d", i)
		if err := svc.Enforce(sub.Id, 0, ip); err != nil {
			t.Fatalf("MaxIps=0 should never reject (ip=%s): %v", ip, err)
		}
	}
}

func TestCheckOnlyDoesNotBind(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 5)
	svc := NewIPBindingService()

	if err := svc.CheckOnly(sub.Id, sub.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("quota available, fresh IP should pass: %v", err)
	}

	var count int64
	database.GetDB().Model(&model.IpBinding{}).Where("subscription_id = ?", sub.Id).Count(&count)
	if count != 0 {
		t.Fatalf("CheckOnly must not insert; got count=%d", count)
	}
}

func TestCheckOnlyRespectsQuotaForUnknownIP(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 1)
	svc := NewIPBindingService()

	// First, bind a known IP via Enforce.
	if err := svc.Enforce(sub.Id, sub.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("bind: %v", err)
	}

	// CheckOnly for the bound IP should pass.
	if err := svc.CheckOnly(sub.Id, sub.MaxIps, "1.1.1.1"); err != nil {
		t.Fatalf("known IP should pass CheckOnly: %v", err)
	}

	// CheckOnly for an unknown IP should fail because quota = 1 is full.
	if err := svc.CheckOnly(sub.Id, sub.MaxIps, "2.2.2.2"); err != ErrIPLimitExceeded {
		t.Fatalf("unknown IP over quota err = %v, want ErrIPLimitExceeded", err)
	}
}

func TestCheckOnlyUnlimited(t *testing.T) {
	setupTestDB(t)
	sub := seedSubscription(t, 1)
	svc := NewIPBindingService()

	if err := svc.CheckOnly(sub.Id, 0, "9.9.9.9"); err != nil {
		t.Fatalf("MaxIps=0 should always pass: %v", err)
	}
}
