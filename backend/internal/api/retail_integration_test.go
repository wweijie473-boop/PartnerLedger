package api

import (
	"fmt"
	"testing"

	"github.com/wweijie473-boop/PartnerLedger/backend/internal/data"
	"gorm.io/gorm"
)

func testRetailWorkflow(t *testing.T, db *gorm.DB, request func(string, string, string, string, int, any),
	product data.Product, dealer, customer, vendor, otherDealer, otherVendor string, dealerID, customerID uint64) {
	t.Helper()
	const base = "/api/retail/v1"
	t.Cleanup(func() {
		for _, cleanup := range []struct {
			sql string
			id  uint64
		}{
			{"DELETE FROM pl_retail_orders WHERE customer_id = ?", customerID},
			{"DELETE FROM pl_retail_promotions WHERE reseller_id = ?", dealerID},
			{"DELETE FROM pl_retail_offers WHERE product_id = ?", product.ID},
		} {
			if err := db.Exec(cleanup.sql, cleanup.id).Error; err != nil {
				t.Error(err)
			}
		}
	})
	offerURL := fmt.Sprintf("%s/manufacturer-offers/%d", base, product.ID)
	price := func(rate int) string {
		return fmt.Sprintf(`{"unit_cents":101,"commission_bps":%d,"active":true}`, rate)
	}
	request("PUT", offerURL, otherVendor, price(5000), 404, nil)
	request("PUT", offerURL, vendor, price(5000), 200, nil)

	var promo, replayPromo retailPromotion
	createCode := fmt.Sprintf(`{"manufacturer_id":"%d"}`, product.ManufacturerID)
	request("POST", base+"/promotion", dealer, createCode, 200, &promo)
	request("POST", base+"/promotion", dealer, createCode, 200, &replayPromo)
	if !retailCode.MatchString(promo.Code) || replayPromo.Code != promo.Code {
		t.Fatal("promotion code must be stable")
	}
	var store struct{ Items []offerCard }
	request("GET", base+"/store/"+promo.Code, "", "", 200, &store)
	if len(store.Items) != 1 || store.Items[0].ProductID != product.ID {
		t.Fatal("promotion must show the correct vendor offer")
	}

	const first = "a1070000-0000-4000-8000-000000000001"
	const second = "a1070000-0000-4000-8000-000000000002"
	direct := fmt.Sprintf(`{"product_id":"%d","code":%q,"quantity":3,"expected_unit_cents":101,"request_id":%q}`, product.ID, promo.Code, first)
	assisted := fmt.Sprintf(`{"product_id":"%d","customer_id":"%d","quantity":1,"expected_unit_cents":101,"request_id":%q}`, product.ID, customerID, second)
	var self, replay, proxy struct{ ID string }
	request("POST", base+"/orders", customer, direct, 200, &self)
	request("POST", base+"/orders", customer, direct, 200, &replay)
	if self.ID == "" || replay.ID != self.ID {
		t.Fatal("duplicate customer submission created another order")
	}
	request("POST", base+"/orders", otherDealer, assisted, 404, nil)
	request("POST", base+"/orders", dealer, assisted, 200, &proxy)
	row := func(order, action string) string { return base + "/orders/" + order + "/" + action }
	request("POST", row(proxy.ID, "receipt"), vendor, `{"reference":"fixture-early"}`, 409, nil)
	request("POST", row(proxy.ID, "confirm"), customer, `{}`, 200, nil)
	request("POST", row(self.ID, "receipt"), otherVendor, `{"reference":"fixture-receipt"}`, 404, nil)
	request("PUT", offerURL, vendor, price(3000), 200, nil)
	receipt := fmt.Sprintf(`{"reference":"fixture-receipt-%d"}`, customerID)
	request("POST", row(self.ID, "receipt"), vendor, receipt, 200, nil)
	request("POST", row(self.ID, "receipt"), vendor, receipt, 200, nil)
	request("POST", row(proxy.ID, "receipt"), vendor, receipt, 409, nil)
	request("POST", row(self.ID, "remind"), otherDealer, `{}`, 404, nil)
	request("POST", row(self.ID, "remind"), dealer, `{}`, 200, nil)
	request("POST", row(self.ID, "remind"), dealer, `{}`, 200, nil)
	var list struct{ Items []retailOrderRow }
	request("GET", base+"/orders", dealer, "", 200, &list)
	if len(list.Items) != 2 {
		t.Fatalf("expected two orders, found %d", len(list.Items))
	}
	order := list.Items[1]
	if order.Status != "settlement" || order.Commission == nil || order.Commission.ReminderCount != 1 ||
		order.Commission.Cents != 152 || order.Commission.RateBPS != 5000 || order.TotalCents != 303 {
		t.Fatal("payment must use the locked price and commission; reminders must be deduplicated")
	}
	request("POST", row(self.ID, "settle"), vendor, `{"reference":"fixture-payout"}`, 200, nil)
	list.Items = nil
	request("GET", base+"/orders", customer, "", 200, &list)
	if list.Items[1].Status != "paid" || list.Items[1].Commission != nil || list.Items[1].Receipt != nil || list.Items[1].Payout != nil {
		t.Fatal("customers should see their payment status without the partner commission")
	}
	request("GET", base+"/orders?order_id="+proxy.ID, customer, "", 200, &list)
	if len(list.Items) != 1 || fmt.Sprint(list.Items[0].ID) != proxy.ID {
		t.Fatal("the assigned customer should be able to open their assisted order directly")
	}
	request("GET", base+"/orders?order_id="+proxy.ID, otherDealer, "", 200, &list)
	if len(list.Items) != 0 {
		t.Fatal("an order ID must not bypass organization scope")
	}
	request("GET", base+"/orders", vendor, "", 200, &list)
	if list.Items[1].Status != "settled" || list.Items[1].Commission.SettledAt == nil || list.Items[1].Receipt == nil || list.Items[1].Payout == nil {
		t.Fatal("settlement missing")
	}
	var funds struct {
		Items   []retailOrderRow
		Summary retailSettlementSummary
	}
	request("GET", base+"/orders?view=commission&status=settlement", dealer, "", 200, &funds)
	if len(funds.Items) != 0 || funds.Summary.SettledCents != 152 || funds.Summary.SettlementCents != 0 || funds.Summary.Reminded != 0 {
		t.Fatal("commission totals must stay scoped and include settled rows even while the list filters them out")
	}
}
