package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type campaignHarness struct {
	t       *testing.T
	a       *App
	h       http.Handler
	session string
	csrf    string
}

func newCampaignHarness(t *testing.T, role string) *campaignHarness {
	a := mustApp(t)
	fixed := time.Date(2026, 10, 7, 12, 0, 0, 0, india)
	a.Now = func() time.Time { return fixed }
	staff, _ := addStaff(t, a, role+"-campaigns@bioconnect.test", role)
	session, csrf := randomToken(), randomToken()
	if _, err := a.DB.Exec(context.Background(), `INSERT INTO sessions(token_hash,staff_id,csrf_hash,expires_at)
		VALUES($1,$2,$3,now()+interval '1 hour')`, hash(session), staff, hash(csrf)); err != nil {
		t.Fatal(err)
	}
	return &campaignHarness{t, a, a.Handler(), session, csrf}
}

func (c *campaignHarness) request(method, path string, body any) *httptest.ResponseRecorder {
	c.t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrf)
	req.AddCookie(&http.Cookie{Name: "bc_session", Value: c.session})
	rr := httptest.NewRecorder()
	c.h.ServeHTTP(rr, req)
	return rr
}

func (c *campaignHarness) audience(body map[string]any) int {
	c.t.Helper()
	rr := c.request("POST", "/api/v1/admin/campaigns/audience", body)
	if rr.Code != 200 {
		c.t.Fatalf("audience returned %d: %s", rr.Code, rr.Body.String())
	}
	var out struct {
		Count  int
		Sample []map[string]any `json:"sample"`
	}
	json.Unmarshal(rr.Body.Bytes(), &out)
	// The console reads these exact lower-case keys.
	for _, r := range out.Sample {
		if email, _ := r["email"].(string); !strings.Contains(email, "@") {
			c.t.Fatalf("sample recipient without email: %s", rr.Body.String())
		}
	}
	return out.Count
}

// seedCampaignAudience creates: two approved delegates (one listed twice with
// different capitalisation), an unpaid delegate, and an approved two-person
// exhibitor team where one representative checked in.
func seedCampaignAudience(t *testing.T, a *App) {
	t.Helper()
	ctx := context.Background()
	sid, _ := addStaff(t, a, "approver@bioconnect.test", "reviewer")
	approve := func(rid string) {
		payDelegate(t, a, rid, dueNow(t, a, rid))
		if err := tryApprove(a, rid, sid, "approve_send", "SBI"+rid[:6], dueNow(t, a, rid)); err != nil {
			t.Fatal(err)
		}
	}
	for i, email := range []string{"asha@example.com", "bina@example.com", "ASHA@example.com"} {
		in := delegateInput("industry")
		in.Email, in.Attendees[0].Email = email, email
		rid, _, err := a.Create(ctx, in, key(700+i), nil)
		if err != nil {
			t.Fatal(err)
		}
		approve(rid)
	}
	unpaid := delegateInput("student")
	unpaid.Email, unpaid.Attendees[0].Email = "unpaid@example.com", "unpaid@example.com"
	if _, _, err := a.Create(ctx, unpaid, key(710), nil); err != nil {
		t.Fatal(err)
	}
	rid, _, err := a.Create(ctx, exhibitorInput("table", 2), key(720), tinyPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	approve(rid)
	if _, err := a.DB.Exec(ctx, `INSERT INTO ops_attendance(attendee_id,event_day,checked_in_by) SELECT id,'2026-10-08','test' FROM attendees WHERE email='rep0@example.com'`); err != nil {
		t.Fatal(err)
	}
}

func approvedAudience(extra map[string]any) map[string]any {
	aud := map[string]any{"statuses": []string{"approved"}, "pass_holders": true, "exclude_previous": true}
	for k, v := range extra {
		aud[k] = v
	}
	return map[string]any{"template_id": "app-launch", "audience": aud}
}

func TestCampaignAudienceFilters(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	cases := []struct {
		name string
		body map[string]any
		want int
	}{
		{"approved pass holders, duplicates merged", approvedAudience(nil), 4},
		{"delegates only", approvedAudience(map[string]any{"kinds": []string{"delegate"}}), 2},
		{"exhibitors only", approvedAudience(map[string]any{"kinds": []string{"exhibitor"}}), 2},
		{"one category", approvedAudience(map[string]any{"categories": []string{"table"}}), 2},
		{"checked in", approvedAudience(map[string]any{"attendance": "attended"}), 1},
		{"not checked in", approvedAudience(map[string]any{"attendance": "absent"}), 3},
		{"awaiting payment", map[string]any{"template_id": "app-launch", "audience": map[string]any{"statuses": []string{"awaiting_payment"}}}, 1},
	}
	for _, tc := range cases {
		if got := c.audience(tc.body); got != tc.want {
			t.Errorf("%s: %d recipients, want %d", tc.name, got, tc.want)
		}
	}
	for _, bad := range []map[string]any{
		{"template_id": "app-launch", "audience": map[string]any{}},
		{"template_id": "app-launch", "audience": map[string]any{"statuses": []string{"paid"}}},
		{"template_id": "app-launch", "audience": map[string]any{"statuses": []string{"approved"}, "categories": []string{"nope"}}},
		{"template_id": "invitation", "audience": map[string]any{"statuses": []string{"approved"}}},
	} {
		if rr := c.request("POST", "/api/v1/admin/campaigns/audience", bad); rr.Code != 400 {
			t.Errorf("invalid audience %v returned %d", bad, rr.Code)
		}
	}
}

func TestCampaignSendQueueAndHistory(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	ctx := context.Background()
	body := approvedAudience(nil)

	body["expected_count"] = 3
	if rr := c.request("POST", "/api/v1/admin/campaigns", body); rr.Code != 409 {
		t.Fatalf("stale count returned %d, want 409", rr.Code)
	}
	body["expected_count"] = 4
	rr := c.request("POST", "/api/v1/admin/campaigns", body)
	if rr.Code != 201 {
		t.Fatalf("create returned %d: %s", rr.Code, rr.Body.String())
	}
	var created struct{ ID string }
	json.Unmarshal(rr.Body.Bytes(), &created)
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE campaign_id=$1 AND status='queued'", created.ID); n != 4 {
		t.Fatalf("%d queued recipients, want 4", n)
	}
	if n := count(t, c.a, "SELECT count(*) FROM audit_events WHERE action='campaign_created'"); n != 1 {
		t.Fatal("campaign creation not audited")
	}
	// The same email cannot go to the same people twice by accident.
	if got := c.audience(approvedAudience(nil)); got != 0 {
		t.Fatalf("exclude previous left %d recipients", got)
	}
	if got := c.audience(approvedAudience(map[string]any{"exclude_previous": false})); got != 4 {
		t.Fatalf("without exclusion got %d", got)
	}

	for {
		worked, err := c.a.CampaignWorkOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE campaign_id=$1 AND status='delivered'", created.ID); n != 4 {
		t.Fatalf("%d delivered, want 4", n)
	}

	rr = c.request("GET", "/api/v1/admin/campaigns/"+created.ID+"?status=delivered&q=asha@", nil)
	if rr.Code != 200 {
		t.Fatalf("detail returned %d", rr.Code)
	}
	var detail struct {
		Campaign   map[string]any
		Recipients []map[string]any
		Matched    int
	}
	json.Unmarshal(rr.Body.Bytes(), &detail)
	if detail.Matched != 1 || detail.Recipients[0]["email"] != "asha@example.com" || detail.Campaign["delivered"] != float64(4) {
		t.Fatalf("unexpected detail: %s", rr.Body.String())
	}
	rr = c.request("GET", "/api/v1/admin/campaigns", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), created.ID) || !strings.Contains(rr.Body.String(), "Mobile app announcement") {
		t.Fatalf("list returned %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCampaignCancelStopsUnsentMessages(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	body := approvedAudience(nil)
	body["expected_count"] = 4
	var created struct{ ID string }
	json.Unmarshal(c.request("POST", "/api/v1/admin/campaigns", body).Body.Bytes(), &created)
	if _, err := c.a.CampaignWorkOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	rr := c.request("POST", "/api/v1/admin/campaigns/"+created.ID+"/cancel", map[string]any{})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), `"cancelled":3`) {
		t.Fatalf("cancel returned %d: %s", rr.Code, rr.Body.String())
	}
	if worked, _ := c.a.CampaignWorkOnce(context.Background()); worked {
		t.Fatal("worker sent a message after cancellation")
	}
	if rr := c.request("POST", "/api/v1/admin/campaigns/"+created.ID+"/cancel", map[string]any{}); rr.Code != 409 {
		t.Fatalf("second cancel returned %d", rr.Code)
	}
	// Cancelled recipients may be sent the email again by a later campaign.
	if got := c.audience(approvedAudience(nil)); got != 3 {
		t.Fatalf("after cancel, %d recipients eligible, want 3", got)
	}
}

func TestCampaignLiveDeliveryPayloadRetryAndWebhook(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	ctx := context.Background()
	var mu sync.Mutex
	var payloads []map[string]any
	fail429 := true
	postmark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p)
		mu.Lock()
		defer mu.Unlock()
		payloads = append(payloads, p)
		if fail429 {
			fail429 = false
			w.WriteHeader(429)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ErrorCode": 0, "MessageID": "pm-" + strings.Split(p["To"].(string), "@")[0]})
	}))
	defer postmark.Close()
	c.a.Config.LiveDelivery = true
	c.a.Config.PostmarkAPIBase = postmark.URL
	c.a.Config.PostmarkToken = "token"
	c.a.Config.SenderAddress = "events@bioconnect.test"
	c.a.Config.SenderName = "Bio Connect 4.0"
	c.a.Config.PostmarkBroadcastStream = "broadcast"
	c.a.Config.WebhookUser, c.a.Config.WebhookPassword = "hook", "s3cret"

	body := approvedAudience(map[string]any{"kinds": []string{"delegate"}})
	body["expected_count"] = 2
	var created struct{ ID string }
	json.Unmarshal(c.request("POST", "/api/v1/admin/campaigns", body).Body.Bytes(), &created)

	// The first attempt is rate limited and waits for a retry.
	if _, err := c.a.CampaignWorkOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE status='queued' AND attempts=1 AND available_at>now()"); n != 1 {
		t.Fatal("rate-limited message not rescheduled")
	}
	c.a.DB.Exec(ctx, "UPDATE campaign_recipients SET available_at=now()")
	for {
		worked, err := c.a.CampaignWorkOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE status='accepted' AND provider_id LIKE 'pm-%'"); n != 2 {
		t.Fatalf("%d accepted, want 2\n%s", n, dumpRecipients(t, c.a))
	}
	last := payloads[len(payloads)-1]
	meta := last["Metadata"].(map[string]any)
	if last["MessageStream"] != "broadcast" || meta["application"] != "bioconnect4" || meta["campaign_id"] != created.ID || len(last["Attachments"].([]any)) != 11 {
		t.Fatalf("bad payload: stream=%v meta=%v", last["MessageStream"], meta)
	}
	if html := last["HtmlBody"].(string); !strings.Contains(html, last["To"].(string)) || !strings.Contains(html, "{{{ pm:unsubscribe }}}") {
		t.Fatal("email not personalised or missing unsubscribe")
	}

	for _, ev := range []string{`{"RecordType":"Delivery","MessageID":"pm-asha","Metadata":{"application":"bioconnect4"}}`, `{"RecordType":"Bounce","MessageID":"pm-bina","Metadata":{"application":"bioconnect4"}}`} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/v1/webhooks/postmark", strings.NewReader(ev))
		req.SetBasicAuth("hook", "s3cret")
		c.a.postmarkWebhook(rr, req)
		if rr.Code != 200 {
			t.Fatalf("webhook returned %d", rr.Code)
		}
	}
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE (email='asha@example.com' AND status='delivered') OR (email='bina@example.com' AND status='failed')"); n != 2 {
		t.Fatal("webhooks did not update campaign recipients")
	}
}

func TestCampaignUncertainSendIsNeverRetried(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	sends := 0
	postmark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends++
		w.WriteHeader(503)
	}))
	defer postmark.Close()
	c.a.Config.LiveDelivery, c.a.Config.PostmarkAPIBase = true, postmark.URL
	body := approvedAudience(map[string]any{"categories": []string{"table"}})
	body["expected_count"] = 2
	c.request("POST", "/api/v1/admin/campaigns", body)
	for i := 0; i < 5; i++ {
		c.a.CampaignWorkOnce(context.Background())
	}
	if sends != 2 || count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE status='uncertain'") != 2 {
		t.Fatalf("%d sends; uncertain messages must not be retried", sends)
	}
}

func TestCampaignTemplateExpiryTestSendPreviewAndRole(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	rr := c.request("POST", "/api/v1/admin/campaigns/test", map[string]any{"template_id": "app-launch"})
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "reviewer-campaigns@bioconnect.test") {
		t.Fatalf("test send returned %d: %s", rr.Code, rr.Body.String())
	}
	if n := count(t, c.a, "SELECT count(*) FROM audit_events WHERE action='campaign_test_sent'"); n != 1 {
		t.Fatal("test send not audited")
	}
	rr = c.request("GET", "/api/v1/admin/campaigns/templates/app-launch/preview", nil)
	if rr.Code != 200 || !strings.Contains(rr.Body.String(), "data:image/jpeg;base64,") || strings.Contains(rr.Body.String(), "cid:") {
		t.Fatalf("preview returned %d", rr.Code)
	}
	if rr.Header().Get("X-Frame-Options") != "SAMEORIGIN" || !strings.Contains(rr.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Fatal("preview must be frameable only by the console, sandboxed")
	}

	// Queued messages for an email past its send-by date are cancelled, not sent.
	seedCampaignAudience(t, c.a)
	body := approvedAudience(nil)
	body["expected_count"] = 4
	c.request("POST", "/api/v1/admin/campaigns", body)
	c.a.Now = func() time.Time { return time.Date(2026, 10, 10, 0, 0, 0, 0, india) }
	c.a.CampaignWorkOnce(context.Background())
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE status='cancelled' AND error_code='template_expired'"); n != 1 {
		t.Fatal("expired template was not cancelled")
	}
	if rr := c.request("POST", "/api/v1/admin/campaigns/audience", approvedAudience(nil)); rr.Code != 400 {
		t.Fatalf("expired template audience returned %d", rr.Code)
	}

	m := newCampaignHarness(t, "manager")
	if rr := m.request("GET", "/api/v1/admin/campaigns", nil); rr.Code != 403 {
		t.Fatalf("manager got %d, want 403", rr.Code)
	}
}

func TestCampaignAttachesEachRecipientsOwnPass(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	ctx := context.Background()
	var mu sync.Mutex
	sent := map[string]map[string]any{}
	postmark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		json.NewDecoder(r.Body).Decode(&p)
		mu.Lock()
		sent[p["To"].(string)] = p
		mu.Unlock()
		json.NewEncoder(w).Encode(map[string]any{"ErrorCode": 0, "MessageID": "pm-" + id()})
	}))
	defer postmark.Close()
	c.a.Config.LiveDelivery, c.a.Config.PostmarkAPIBase = true, postmark.URL

	reminder := func(extra map[string]any) map[string]any {
		b := approvedAudience(extra)
		b["template_id"] = "event-reminder"
		return b
	}
	if rr := c.request("POST", "/api/v1/admin/campaigns/audience", reminder(map[string]any{"pass_holders": false})); rr.Code != 400 {
		t.Fatalf("pass email without pass holders returned %d", rr.Code)
	}

	// Test sends carry a sample, never a real pass.
	rr := c.request("POST", "/api/v1/admin/campaigns/test", map[string]any{"template_id": "event-reminder"})
	if rr.Code != 200 {
		t.Fatalf("test send returned %d: %s", rr.Code, rr.Body.String())
	}
	if !hasAttachment(sent["reviewer-campaigns@bioconnect.test"], "Bio-Connect-4.0-pass-TEST-0000.pdf") {
		t.Fatal("test email missing the sample pass")
	}

	body := reminder(nil)
	body["expected_count"] = 4
	rr = c.request("POST", "/api/v1/admin/campaigns", body)
	if rr.Code != 201 {
		t.Fatalf("create returned %d: %s", rr.Code, rr.Body.String())
	}
	// A pass revoked after the snapshot is not sent.
	if _, err := c.a.DB.Exec(ctx, "UPDATE passes SET revoked_at=now() WHERE attendee_id=(SELECT id FROM attendees WHERE email='rep1@example.com')"); err != nil {
		t.Fatal(err)
	}
	for {
		worked, err := c.a.CampaignWorkOnce(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	for _, email := range []string{"asha@example.com", "bina@example.com", "rep0@example.com"} {
		var number string
		if err := c.a.DB.QueryRow(ctx, `SELECT p.number FROM passes p JOIN campaign_recipients cr ON cr.pass_id=p.id WHERE cr.email=$1`, email).Scan(&number); err != nil {
			t.Fatal(err)
		}
		if !hasAttachment(sent[email], "Bio-Connect-4.0-pass-"+number+".pdf") {
			t.Fatalf("%s did not receive their own pass %s: %v\n%s", email, number, attachmentNames(sent[email]), dumpRecipients(t, c.a))
		}
	}
	if _, ok := sent["rep1@example.com"]; ok {
		t.Fatal("revoked pass was sent")
	}
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE email='rep1@example.com' AND status='cancelled' AND error_code='pass_revoked'"); n != 1 {
		t.Fatal("revoked pass recipient not cancelled")
	}
}

func hasAttachment(payload map[string]any, name string) bool {
	list, _ := payload["Attachments"].([]any)
	for _, a := range list {
		if m, _ := a.(map[string]any); m["Name"] == name && m["ContentType"] == "application/pdf" && m["Content"] != "" {
			return true
		}
	}
	return false
}

func attachmentNames(payload map[string]any) []any {
	var out []any
	list, _ := payload["Attachments"].([]any)
	for _, a := range list {
		m, _ := a.(map[string]any)
		out = append(out, m["Name"])
	}
	return out
}

func dumpRecipients(t *testing.T, a *App) string {
	rows, err := a.DB.Query(context.Background(), "SELECT email,status,attempts,error_code,available_at>now(),available_at,now() FROM campaign_recipients ORDER BY email")
	if err != nil {
		return err.Error()
	}
	defer rows.Close()
	var b strings.Builder
	for rows.Next() {
		v, _ := rows.Values()
		fmt.Fprintln(&b, v...)
	}
	return b.String()
}

func TestCampaignSendWindowOpensOnTheDay(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	body := approvedAudience(nil)
	body["template_id"] = "event-today"
	// The evening before: preview, count and test, but no send.
	c.a.Now = func() time.Time { return time.Date(2026, 10, 7, 23, 59, 0, 0, india) }
	if got := c.audience(body); got != 4 {
		t.Fatalf("audience before opening: %d", got)
	}
	if rr := c.request("POST", "/api/v1/admin/campaigns/test", map[string]any{"template_id": "event-today"}); rr.Code != 200 {
		t.Fatalf("test before opening returned %d: %s", rr.Code, rr.Body.String())
	}
	body["expected_count"] = 4
	if rr := c.request("POST", "/api/v1/admin/campaigns", body); rr.Code != 400 || !strings.Contains(rr.Body.String(), "can be sent from 8 October 2026, 00:00 IST") {
		t.Fatalf("send before opening returned %d: %s", rr.Code, rr.Body.String())
	}
	rr := c.request("GET", "/api/v1/admin/campaigns", nil)
	if !strings.Contains(rr.Body.String(), `"id":"event-today"`) {
		t.Fatal("upcoming email missing from the list")
	}
	// On the day it sends; the next day it is gone.
	c.a.Now = func() time.Time { return time.Date(2026, 10, 8, 15, 0, 0, 0, india) }
	if rr := c.request("POST", "/api/v1/admin/campaigns", body); rr.Code != 201 {
		t.Fatalf("send on the day returned %d: %s", rr.Code, rr.Body.String())
	}
	c.a.Now = func() time.Time { return time.Date(2026, 10, 9, 0, 0, 0, 0, india) }
	if rr := c.request("POST", "/api/v1/admin/campaigns/test", map[string]any{"template_id": "event-today"}); rr.Code != 400 {
		t.Fatalf("test after expiry returned %d", rr.Code)
	}
}

// Postmark refuses the whole message (422, ErrorCode 300) when a metadata name
// exceeds 20 characters or a value 80. This fake enforces the same limits.
func TestCampaignMetadataFitsPostmarkLimits(t *testing.T) {
	c := newCampaignHarness(t, "reviewer")
	seedCampaignAudience(t, c.a)
	postmark := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p struct{ Metadata map[string]string }
		json.NewDecoder(r.Body).Decode(&p)
		for k, v := range p.Metadata {
			if len(k) > 20 || len(v) > 80 {
				w.WriteHeader(422)
				json.NewEncoder(w).Encode(map[string]any{"ErrorCode": 300, "Message": "Invalid metadata content."})
				return
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"ErrorCode": 0, "MessageID": "pm-" + id()})
	}))
	defer postmark.Close()
	c.a.Config.LiveDelivery, c.a.Config.PostmarkAPIBase = true, postmark.URL
	if rr := c.request("POST", "/api/v1/admin/campaigns/test", map[string]any{"template_id": "app-launch"}); rr.Code != 200 {
		t.Fatalf("test send rejected: %d %s", rr.Code, rr.Body.String())
	}
	body := approvedAudience(nil)
	body["expected_count"] = 4
	c.request("POST", "/api/v1/admin/campaigns", body)
	for {
		worked, err := c.a.CampaignWorkOnce(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !worked {
			break
		}
	}
	if n := count(t, c.a, "SELECT count(*) FROM campaign_recipients WHERE status='accepted'"); n != 4 {
		t.Fatalf("%d accepted, want 4\n%s", n, dumpRecipients(t, c.a))
	}
}

func TestPostmarkRejectionKeepsErrorCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"ErrorCode":300,"Message":"Invalid metadata content."}`))
	}))
	defer srv.Close()
	if r := providerRequest(context.Background(), srv.URL, "X-Postmark-Server-Token", "t", map[string]string{}, "email"); r.Status != "failed" || r.Code != "postmark_300" {
		t.Fatalf("got %+v", r)
	}
}
