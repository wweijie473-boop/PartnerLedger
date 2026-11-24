package api

import (
	"fmt"
	"testing"
	"time"

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
			{"DELETE FROM pl_dealer_customer_profiles WHERE reseller_id = ?", dealerID},
		} {
			if err := db.Exec(cleanup.sql, cleanup.id).Error; err != nil {
				t.Error(err)
			}
		}
	})
	offerURL := fmt.Sprintf("%s/manufacturer-offers/%d", base, product.ID)
	price := func(rate int) string {
		return fmt.Sprintf(`{"unit_cents":101,"commission_bps":%d,"active":true,"annual":true}`, rate)
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
	if len(store.Items) != 1 || store.Items[0].ProductID != product.ID || !store.Items[0].Annual {
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
	const ops = base + "/operations"
	var renewals struct{ Items []renewalEntry }
	request("GET", ops+"/renewals", dealer, "", 200, &renewals)
	if len(renewals.Items) != 0 {
		t.Fatal("unpaid order appeared in renewals")
	}
	row := func(order, action string) string { return base + "/orders/" + order + "/" + action }
	request("POST", row(proxy.ID, "receipt"), vendor, `{"reference":"fixture-early"}`, 409, nil)
	request("POST", row(proxy.ID, "confirm"), customer, `{}`, 200, nil)
	request("POST", row(self.ID, "receipt"), otherVendor, `{"reference":"fixture-receipt"}`, 404, nil)
	request("PUT", offerURL, vendor, `{"unit_cents":101,"commission_bps":3000,"active":true,"annual":false}`, 200, nil)
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

	const profile = `{"name":"Fixture prospect","contact":"Fixture contact","note":"Follow up"}`
	request("GET", ops+"/overview", vendor, "", 403, nil)
	request("GET", ops+"/customers", customer, "", 403, nil)
	request("POST", ops+"/customers", dealer, profile, 200, nil)
	request("POST", ops+"/customers", dealer, profile, 200, nil)
	request("POST", ops+"/customers", dealer, `{"name":"Fixture prospect","note":"different"}`, 409, nil)
	organizationKey := fmt.Sprintf("o%d", customerID)
	request("PUT", ops+"/customers/"+organizationKey, otherDealer, `{"note":"Out of scope"}`, 404, nil)
	request("PUT", ops+"/customers/"+organizationKey, dealer, `{"name":"Renamed organization"}`, 409, nil)
	request("PUT", ops+"/customers/"+organizationKey, dealer, `{"contact":"Current contact"}`, 200, nil)
	var contacts struct{ Items []operationsCustomer }
	request("GET", ops+"/customers", dealer, "", 200, &contacts)
	if len(contacts.Items) != 2 || contacts.Items[0].ID != organizationKey || !contacts.Items[0].Linked ||
		contacts.Items[0].Contact != "Current contact" || !contacts.Items[1].Prospect || contacts.Items[1].Linked {
		t.Fatalf("unexpected customer list: %+v", contacts.Items)
	}
	request("PUT", ops+"/customers/"+contacts.Items[1].ID, dealer, `{"name":"Updated fixture","contact":"Contact"}`, 200, nil)
	request("GET", ops+"/customers?after="+organizationKey, dealer, "", 200, &contacts)
	if len(contacts.Items) != 1 || contacts.Items[0].Name != "Updated fixture" {
		t.Fatal("customer cursor or update failed")
	}
	request("GET", ops+"/customers", otherDealer, "", 200, &contacts)
	if len(contacts.Items) != 0 {
		t.Fatal("customer records leaked to another dealer")
	}

	request("PUT", ops+"/renewals/"+proxy.ID, dealer, `{"note":"Unpaid"}`, 404, nil)
	request("PUT", ops+"/renewals/"+self.ID, otherDealer, `{"note":"Out of scope"}`, 404, nil)
	request("PUT", ops+"/renewals/"+self.ID, dealer, `{"note":"Discussed renewal"}`, 200, nil)
	request("PUT", ops+"/renewals/"+self.ID, dealer, `{"note":"Discussed renewal"}`, 200, nil)
	request("GET", ops+"/renewals", dealer, "", 200, &renewals)
	if len(renewals.Items) != 1 || fmt.Sprint(renewals.Items[0].ID) != self.ID ||
		renewals.Items[0].DueAt != annualExpiry(renewals.Items[0].PaidAt) ||
		renewals.Items[0].Note != "Discussed renewal" || renewals.Items[0].ContactedAt == nil {
		t.Fatalf("annual order snapshot or renewal note missing: %+v", renewals.Items)
	}
	request("GET", ops+"/renewals", otherDealer, "", 200, &renewals)
	if len(renewals.Items) != 0 {
		t.Fatal("renewals leaked to another dealer")
	}
	var totals operationsTotals
	request("GET", ops+"/overview", dealer, "", 200, &totals)
	if totals.Customers != 2 || totals.Orders != 2 || totals.PromotionOrders != 1 ||
		totals.PendingCommissionCents != 0 || totals.Renewals30Days != 0 {
		t.Fatalf("unexpected operations totals: %+v", totals)
	}
	testChannelFinanceAndSupport(t, db, request, product.ManufacturerID, dealerID, self.ID, proxy.ID, dealer, vendor, customer, otherDealer, otherVendor)
}

func TestAnnualExpiryUsesLocalCalendar(t *testing.T) {
	paid := time.Date(2024, time.February, 28, 19, 0, 0, 0, time.UTC)
	if got := annualExpiry(paid); got != "2025-02-28" {
		t.Fatalf("expected last day in February after leap-day purchase, got %s", got)
	}
}
