package api

import (
	"fmt"
	"testing"

	"gorm.io/gorm"
)

func testChannelFinanceAndSupport(t *testing.T, db *gorm.DB, request func(string, string, string, string, int, any),
	manufacturerID, dealerID uint64, paidOrder, unpaidOrder, dealer, vendor, customer, otherDealer, otherVendor string) {
	t.Helper()
	const ops = "/api/retail/v1/operations"
	t.Cleanup(func() {
		for _, table := range []string{"pl_channel_service_tickets", "pl_channel_finance_requests", "pl_channel_service_allowances"} {
			if err := db.Exec("DELETE FROM "+table+" WHERE reseller_id = ? AND manufacturer_id = ?", dealerID, manufacturerID).Error; err != nil {
				t.Error(err)
			}
		}
	})
	maker := fmt.Sprint(manufacturerID)
	allowancePath := fmt.Sprintf("%s/allowances/%d", ops, dealerID)
	finance := ops + "/finance"
	withAmount := func(reference string, cents int) string {
		return fmt.Sprintf(`{"manufacturer_id":%q,"reference":%q,"amount_cents":%d}`, maker, reference, cents)
	}
	invoice := func(order, title string) string {
		return fmt.Sprintf(`{"order_id":%q,"title":%q}`, order, title)
	}

	request("GET", ops+"/partners", customer, "", 403, nil)
	request("POST", finance+"/topups", otherDealer, withAmount("bank-test", 700), 404, nil)
	request("POST", finance+"/topups", dealer, withAmount("bank-test", 700), 200, nil)
	request("POST", finance+"/topups", dealer, withAmount("bank-test", 700), 200, nil)
	request("POST", finance+"/topups", dealer, withAmount("bank-test", 701), 409, nil)
	request("POST", finance+"/invoices", dealer, invoice(unpaidOrder, "Fixture title"), 409, nil)
	request("POST", finance+"/invoices", otherDealer, invoice(paidOrder, "Fixture title"), 404, nil)
	request("POST", finance+"/invoices", dealer, invoice(paidOrder, "Fixture title"), 200, nil)
	request("POST", finance+"/invoices", dealer, invoice(paidOrder, "Another title"), 409, nil)

	request("POST", finance+"/claims", dealer, withAmount("fee-first", 400), 409, nil)
	request("PUT", allowancePath, otherVendor, `{"limit_cents":500}`, 404, nil)
	request("PUT", allowancePath, dealer, `{"limit_cents":500}`, 403, nil)
	request("PUT", allowancePath, vendor, `{"limit_cents":500}`, 200, nil)
	request("POST", finance+"/claims", dealer, withAmount("fee-first", 400), 200, nil)
	request("POST", finance+"/claims", dealer, withAmount("fee-first", 400), 200, nil)
	request("POST", finance+"/claims", dealer, withAmount("fee-second", 101), 409, nil)
	request("PUT", allowancePath, vendor, `{"limit_cents":399}`, 409, nil)
	request("PUT", allowancePath, vendor, `{"limit_cents":600}`, 200, nil)

	var view struct{ Items []channelFinanceRequest }
	request("GET", finance, otherDealer, "", 200, &view)
	if len(view.Items) != 0 {
		t.Fatal("finance visible across dealers")
	}
	request("GET", finance, vendor, "", 200, &view)
	if len(view.Items) != 3 || view.Items[1].Kind != "invoices" || view.Items[1].AmountCents != 303 {
		t.Fatalf("unexpected finance ledger: %+v", view.Items)
	}
	for _, item := range view.Items {
		path := fmt.Sprintf("%s/%s/%d/complete", finance, item.Kind, item.ID)
		request("POST", path, otherVendor, `{"result_reference":"proof"}`, 404, nil)
		request("POST", path, vendor, `{"result_reference":"proof"}`, 200, nil)
		request("POST", path, vendor, `{"result_reference":"proof"}`, 200, nil)
		request("POST", path, vendor, `{"result_reference":"changed-proof"}`, 409, nil)
	}
	var partners struct{ Items []operationsPartner }
	request("GET", ops+"/partners", dealer, "", 200, &partners)
	if len(partners.Items) != 1 || partners.Items[0].LimitCents != 600 || partners.Items[0].PendingCents != 0 || partners.Items[0].PaidCents != 400 {
		t.Fatalf("unexpected balances: %+v", partners.Items)
	}

	const key = "c1070000-0000-4000-8000-000000000001"
	body := fmt.Sprintf(`{"order_id":%q,"category":"授权与交付","subject":"License delivery","detail":"Follow up","request_id":%q}`, paidOrder, key)
	tickets := ops + "/tickets"
	request("POST", tickets, otherDealer, body, 404, nil)
	var ticket, replay struct{ ID string }
	request("POST", tickets, dealer, body, 200, &ticket)
	request("POST", tickets, dealer, body, 200, &replay)
	if ticket.ID == "" || ticket != replay {
		t.Fatal("ticket retry should reuse the previous record")
	}
	ticketURL := tickets + "/" + ticket.ID
	request("POST", ticketURL+"/resolve", otherVendor, `{"revision":1,"reply":"Not our customer"}`, 404, nil)
	request("POST", ticketURL+"/close", dealer, `{"revision":1}`, 409, nil)
	request("POST", ticketURL+"/resolve", vendor, `{"revision":1,"reply":"First response"}`, 200, nil)
	request("POST", ticketURL+"/reopen", dealer, `{"revision":2,"reply":"One more question"}`, 200, nil)
	request("POST", ticketURL+"/resolve", vendor, `{"revision":1,"reply":"First response"}`, 200, nil)
	request("POST", ticketURL+"/resolve", vendor, `{"revision":1,"reply":"Stale edit"}`, 409, nil)
	request("POST", ticketURL+"/resolve", vendor, `{"revision":3,"reply":"Follow-up response"}`, 200, nil)
	var totals operationsTotals
	request("GET", ops+"/overview", dealer, "", 200, &totals)
	if totals.OpenTickets != 1 {
		t.Fatalf("expected one open or resolved ticket, got %d", totals.OpenTickets)
	}
	request("POST", ticketURL+"/close", dealer, `{"revision":4}`, 200, nil)
	request("GET", ops+"/overview", dealer, "", 200, &totals)
	if totals.OpenTickets != 0 {
		t.Fatal("closed ticket included in open count")
	}
	var thread struct{ Items []channelServiceTicket }
	request("GET", tickets, vendor, "", 200, &thread)
	if len(thread.Items) != 1 || thread.Items[0].Status != "closed" || thread.Items[0].Revision != 5 ||
		len(thread.Items[0].Replies) != 3 || thread.Items[0].Replies[1].Role != "dealer" {
		t.Fatalf("unexpected ticket thread: %+v", thread.Items)
	}
	request("GET", tickets, otherVendor, "", 200, &thread)
	if len(thread.Items) != 0 {
		t.Fatal("ticket visible across manufacturers")
	}
	withoutOrder := fmt.Sprintf(`{"manufacturer_id":%q,"category":"产品使用","subject":"General support","detail":"Follow up","request_id":"c1070000-0000-4000-8000-000000000002"}`, maker)
	request("POST", tickets, dealer, withoutOrder, 200, nil)
}
