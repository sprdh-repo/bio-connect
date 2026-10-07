package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"bioconnect/registration/internal/campaigns"

	"github.com/jackc/pgx/v5"
)

// audience selects attendees, never registration contacts: the attendee email
// is the identity My passes and the venue use. Every field narrows the set.
type audience struct {
	Statuses   []string `json:"statuses"`
	Kinds      []string `json:"kinds"`
	Categories []string `json:"categories"`
	// Only attendees holding an active pass.
	PassHolders bool `json:"pass_holders"`
	// "" for everyone, "attended" or "absent" by venue check-in.
	Attendance string `json:"attendance"`
	// Skip addresses that were already sent this template by another campaign.
	ExcludePrevious bool `json:"exclude_previous"`
}

var campaignStatuses = map[string]bool{"awaiting_payment": true, "awaiting_review": true, "correction_requested": true, "approved": true, "rejected": true, "cancelled": true}

func (a *App) validAudience(ctx context.Context, in *audience) error {
	if len(in.Statuses) == 0 {
		return errors.New("choose at least one registration status")
	}
	for _, s := range in.Statuses {
		if !campaignStatuses[s] {
			return fmt.Errorf("unknown registration status %q", s)
		}
	}
	for _, k := range in.Kinds {
		if k != "delegate" && k != "exhibitor" {
			return fmt.Errorf("unknown registration type %q", k)
		}
	}
	if in.Attendance != "" && in.Attendance != "attended" && in.Attendance != "absent" {
		return errors.New("attendance must be attended or absent")
	}
	if len(in.Categories) > 0 {
		var n int
		if e := a.DB.QueryRow(ctx, "SELECT count(*) FROM categories WHERE id=ANY($1)", in.Categories).Scan(&n); e != nil {
			return e
		}
		if n != len(in.Categories) {
			return errors.New("unknown registration category")
		}
	}
	// Empty slices, not nil, so the SQL cardinality checks see an empty array.
	if in.Kinds == nil {
		in.Kinds = []string{}
	}
	if in.Categories == nil {
		in.Categories = []string{}
	}
	return nil
}

// audienceQuery returns one row (email, name) per distinct lower-cased email,
// taking the earliest registration's attendee name for duplicates.
const audienceQuery = `SELECT DISTINCT ON (lower(a.email)) lower(a.email),a.name,
 COALESCE((SELECT p.id FROM passes p WHERE p.attendee_id=a.id AND p.revoked_at IS NULL ORDER BY p.version DESC LIMIT 1),'')
 FROM attendees a JOIN registrations r ON r.id=a.registration_id JOIN categories c ON c.id=r.category_id
 WHERE a.removed_at IS NULL AND a.email<>'' AND r.status=ANY($1)
 AND (cardinality($2::text[])=0 OR c.kind=ANY($2))
 AND (cardinality($3::text[])=0 OR c.id=ANY($3))
 AND (NOT $4 OR EXISTS(SELECT 1 FROM passes p WHERE p.attendee_id=a.id AND p.revoked_at IS NULL))
 AND ($5='' OR ($5='attended')=EXISTS(SELECT 1 FROM ops_attendance oa WHERE oa.attendee_id=a.id))
 AND (NOT $6 OR NOT EXISTS(SELECT 1 FROM campaign_recipients cr JOIN campaigns cp ON cp.id=cr.campaign_id
   WHERE cp.template_id=$7 AND cr.email=lower(a.email) AND cr.status<>'cancelled'))
 ORDER BY lower(a.email),r.created_at,a.position`

type campaignRecipient struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	PassID string `json:"-"`
}

func (a *App) resolveAudience(ctx context.Context, q interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, templateID string, in audience) ([]campaignRecipient, error) {
	rows, e := q.Query(ctx, audienceQuery, in.Statuses, in.Kinds, in.Categories, in.PassHolders, in.Attendance, in.ExcludePrevious, templateID)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []campaignRecipient{}
	for rows.Next() {
		var r campaignRecipient
		if e = rows.Scan(&r.Email, &r.Name, &r.PassID); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// checkAudienceForTemplate enforces template-specific audience rules.
func checkAudienceForTemplate(t campaigns.Template, in audience) error {
	if t.AttachPass && !in.PassHolders {
		return errors.New("this email attaches each person's pass, so it can only go to pass holders")
	}
	return nil
}

// campaignTemplate returns a template staff may use. Previews, audience
// counts and tests only need it unexpired; a real send needs its window open.
func (a *App) campaignTemplate(id string, send bool) (campaigns.Template, error) {
	t, ok := campaigns.Get(id)
	if !ok || !t.Registrants {
		return t, errors.New("choose an email to send")
	}
	if t.Expired(a.Now()) {
		return t, errors.New("this email is past its send-by date")
	}
	if send && !t.Open(a.Now()) {
		return t, errors.New("this email can be sent from " + t.From.In(india).Format("2 January 2006, 15:04") + " IST")
	}
	return t, nil
}

func (a *App) campaignsAPI(w http.ResponseWriter, r *http.Request, p principal, path string) {
	rest := strings.TrimPrefix(strings.TrimPrefix(path, "campaigns"), "/")
	switch {
	case rest == "" && r.Method == "GET":
		a.listCampaigns(w, r)
	case rest == "audience" && r.Method == "POST":
		var in struct {
			TemplateID string   `json:"template_id"`
			Audience   audience `json:"audience"`
		}
		if !decode(w, r, &in) {
			return
		}
		t, e := a.campaignTemplate(in.TemplateID, false)
		if e == nil {
			e = checkAudienceForTemplate(t, in.Audience)
		}
		if e != nil {
			fail(w, 400, e.Error())
			return
		}
		if e := a.validAudience(r.Context(), &in.Audience); e != nil {
			fail(w, 400, e.Error())
			return
		}
		list, e := a.resolveAudience(r.Context(), a.DB, in.TemplateID, in.Audience)
		if e != nil {
			fail(w, 503, "audience unavailable")
			return
		}
		respond(w, 200, map[string]any{"count": len(list), "sample": list[:min(len(list), 5)]})
	case rest == "test" && r.Method == "POST":
		a.sendCampaignTest(w, r, p)
	case rest == "" && r.Method == "POST":
		a.createCampaign(w, r, p)
	case strings.HasPrefix(rest, "templates/") && strings.HasSuffix(rest, "/preview") && r.Method == "GET":
		a.previewCampaignTemplate(w, r, p, strings.TrimSuffix(strings.TrimPrefix(rest, "templates/"), "/preview"))
	case strings.HasSuffix(rest, "/cancel") && r.Method == "POST":
		a.cancelCampaign(w, r, p, strings.TrimSuffix(rest, "/cancel"))
	case rest != "" && !strings.Contains(rest, "/") && r.Method == "GET":
		a.campaignDetail(w, r, rest)
	default:
		fail(w, 404, "not found")
	}
}

const campaignCounts = `count(cr.id) AS total,
 count(cr.id) FILTER (WHERE cr.status IN ('queued','sending')) AS pending,
 count(cr.id) FILTER (WHERE cr.status='accepted') AS accepted,
 count(cr.id) FILTER (WHERE cr.status='delivered') AS delivered,
 count(cr.id) FILTER (WHERE cr.status='failed') AS failed,
 count(cr.id) FILTER (WHERE cr.status='uncertain') AS uncertain,
 count(cr.id) FILTER (WHERE cr.status='cancelled') AS cancelled`

func (a *App) listCampaigns(w http.ResponseWriter, r *http.Request) {
	templates := []map[string]any{}
	for _, t := range campaigns.All() {
		if t.Registrants {
			templates = append(templates, map[string]any{"id": t.ID, "label": t.Label, "description": t.Description, "subject": t.Subject, "send_by": t.Until, "send_from": t.From, "open": t.Open(a.Now()), "expired": t.Expired(a.Now()), "attach_pass": t.AttachPass})
		}
	}
	items, e := a.queryMaps(r, `SELECT cp.id,cp.template_id,cp.subject,cp.audience,cp.created_at,cp.cancelled_at,s.email AS created_by,`+campaignCounts+`
		FROM campaigns cp JOIN staff s ON s.id=cp.created_by LEFT JOIN campaign_recipients cr ON cr.campaign_id=cp.id
		GROUP BY cp.id,s.email ORDER BY cp.created_at DESC LIMIT 100`)
	if e != nil {
		fail(w, 503, "campaigns unavailable")
		return
	}
	labels := map[string]string{}
	for _, t := range campaigns.All() {
		labels[t.ID] = t.Label
	}
	for _, item := range items {
		item["template_label"] = labels[fmt.Sprint(item["template_id"])]
	}
	categories, e := a.queryMaps(r, "SELECT id,kind,label FROM categories ORDER BY kind,label")
	if e != nil {
		fail(w, 503, "campaigns unavailable")
		return
	}
	respond(w, 200, map[string]any{"templates": templates, "campaigns": items, "categories": categories})
}

func (a *App) createCampaign(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		TemplateID string   `json:"template_id"`
		Audience   audience `json:"audience"`
		// The count staff reviewed. A different count at send time means the
		// audience changed in between, so they review it again.
		ExpectedCount int `json:"expected_count"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, e := a.campaignTemplate(in.TemplateID, true)
	if e == nil {
		e = checkAudienceForTemplate(t, in.Audience)
	}
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	if e = a.validAudience(r.Context(), &in.Audience); e != nil {
		fail(w, 400, e.Error())
		return
	}
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	defer tx.Rollback(r.Context())
	// Serialises campaign creation, so two staff confirming the same email at
	// once cannot both pass the exclude-previous check.
	if _, e = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(420062027)"); e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	list, e := a.resolveAudience(r.Context(), tx, in.TemplateID, in.Audience)
	if e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	if len(list) == 0 {
		fail(w, 400, "no recipients match these filters")
		return
	}
	if len(list) != in.ExpectedCount {
		fail(w, 409, fmt.Sprintf("the audience changed to %d recipients; review it and send again", len(list)))
		return
	}
	cid := id()
	audienceJSON, _ := json.Marshal(in.Audience)
	if _, e = tx.Exec(r.Context(), "INSERT INTO campaigns(id,template_id,subject,audience,created_by) VALUES($1,$2,$3,$4,$5)", cid, t.ID, t.Subject, audienceJSON, p.ID); e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	rows := make([][]any, len(list))
	for i, rcp := range list {
		var pass any
		if t.AttachPass && rcp.PassID != "" {
			pass = rcp.PassID
		}
		rows[i] = []any{id(), cid, rcp.Email, rcp.Name, pass}
	}
	if _, e = tx.CopyFrom(r.Context(), pgx.Identifier{"campaign_recipients"}, []string{"id", "campaign_id", "email", "name", "pass_id"}, pgx.CopyFromRows(rows)); e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	if e = audit(r.Context(), tx, p.ID, "", "campaign_created", cid+" template="+t.ID+" recipients="+strconv.Itoa(len(list))+" audience="+string(audienceJSON)); e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 503, "could not create campaign")
		return
	}
	respond(w, 201, map[string]any{"id": cid, "count": len(list)})
}

func (a *App) campaignDetail(w http.ResponseWriter, r *http.Request, cid string) {
	items, e := a.queryMaps(r, `SELECT cp.id,cp.template_id,cp.subject,cp.audience,cp.created_at,cp.cancelled_at,s.email AS created_by,x.email AS cancelled_by,`+campaignCounts+`
		FROM campaigns cp JOIN staff s ON s.id=cp.created_by LEFT JOIN staff x ON x.id=cp.cancelled_by LEFT JOIN campaign_recipients cr ON cr.campaign_id=cp.id
		WHERE cp.id=$1 GROUP BY cp.id,s.email,x.email`, cid)
	if e != nil {
		fail(w, 503, "campaign unavailable")
		return
	}
	if len(items) == 0 {
		fail(w, 404, "campaign not found")
		return
	}
	if t, ok := campaigns.Get(fmt.Sprint(items[0]["template_id"])); ok {
		items[0]["template_label"] = t.Label
	}
	q := r.URL.Query()
	statusFilter := q.Get("status")
	if statusFilter != "" && statusFilter != "pending" && statusFilter != "accepted" && statusFilter != "delivered" && statusFilter != "failed" && statusFilter != "uncertain" && statusFilter != "cancelled" {
		fail(w, 400, "unknown status")
		return
	}
	search := strings.ToLower(strings.TrimSpace(q.Get("q")))
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 || page > 100000 {
		page = 1
	}
	const size = 100
	where := `campaign_id=$1 AND ($2='' OR ($2='pending' AND status IN ('queued','sending')) OR status=$2) AND ($3='' OR strpos(email,$3)>0 OR strpos(lower(name),$3)>0)`
	var matched int
	if e = a.DB.QueryRow(r.Context(), "SELECT count(*) FROM campaign_recipients WHERE "+where, cid, statusFilter, search).Scan(&matched); e != nil {
		fail(w, 503, "campaign unavailable")
		return
	}
	recipients, e := a.queryMaps(r, `SELECT email,name,status,attempts,COALESCE(provider_id,'') AS provider_id,error_code,updated_at
		FROM campaign_recipients WHERE `+where+` ORDER BY email LIMIT $4 OFFSET $5`, cid, statusFilter, search, size, (page-1)*size)
	if e != nil {
		fail(w, 503, "campaign unavailable")
		return
	}
	respond(w, 200, map[string]any{"campaign": items[0], "recipients": recipients, "matched": matched, "page": page, "page_size": size})
}

func (a *App) cancelCampaign(w http.ResponseWriter, r *http.Request, p principal, cid string) {
	tx, e := a.DB.Begin(r.Context())
	if e != nil {
		fail(w, 503, "could not cancel campaign")
		return
	}
	defer tx.Rollback(r.Context())
	tag, e := tx.Exec(r.Context(), "UPDATE campaigns SET cancelled_at=now(),cancelled_by=$2 WHERE id=$1 AND cancelled_at IS NULL", cid, p.ID)
	if e == nil && tag.RowsAffected() != 1 {
		fail(w, 409, "campaign is already cancelled or does not exist")
		return
	}
	var n int64
	if e == nil {
		// Messages already handed to the worker finish; nothing else goes out.
		tag, e = tx.Exec(r.Context(), "UPDATE campaign_recipients SET status='cancelled',error_code='cancelled_by_staff',updated_at=now() WHERE campaign_id=$1 AND status='queued'", cid)
		n = tag.RowsAffected()
	}
	if e == nil {
		e = audit(r.Context(), tx, p.ID, "", "campaign_cancelled", cid+" unsent="+strconv.FormatInt(n, 10))
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		fail(w, 503, "could not cancel campaign")
		return
	}
	respond(w, 200, map[string]any{"cancelled": n})
}

func (a *App) staffEmail(ctx context.Context, staffID string) (string, error) {
	var email string
	e := a.DB.QueryRow(ctx, "SELECT email FROM staff WHERE id=$1", staffID).Scan(&email)
	return email, e
}

// sendCampaignTest sends one copy to the signed-in staff member, outside the
// queue, so they can check it in a real inbox before sending.
func (a *App) sendCampaignTest(w http.ResponseWriter, r *http.Request, p principal) {
	var in struct {
		TemplateID string `json:"template_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, e := a.campaignTemplate(in.TemplateID, false)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	if a.limited(r.Context(), "campaign-test:"+p.ID, 20, time.Hour) {
		fail(w, 429, "too many test emails; try again later")
		return
	}
	email, e := a.staffEmail(r.Context(), p.ID)
	if e != nil {
		fail(w, 503, "test email unavailable")
		return
	}
	var attachments []map[string]string
	if t.AttachPass {
		b, e := SamplePassPDF()
		if e != nil {
			fail(w, 503, "test email unavailable")
			return
		}
		attachments = append(attachments, passAttachment(SamplePassNumber, b))
	}
	result := a.sendCampaignEmail(r.Context(), t, "[TEST] ", email, "", attachments, map[string]string{"campaign_test": p.ID})
	if _, e = a.DB.Exec(r.Context(), "INSERT INTO audit_events(staff_id,action,detail) VALUES($1,'campaign_test_sent',$2)", p.ID, t.ID+" status="+result.Status+" code="+result.Code+" provider_id="+result.ID); e != nil {
		fail(w, 503, "test email unavailable")
		return
	}
	if result.Status != "accepted" && result.Status != "delivered" {
		fail(w, 502, "the test email was not accepted ("+result.Code+")")
		return
	}
	respond(w, 200, map[string]string{"email": email, "provider_id": result.ID})
}

// previewCampaignTemplate serves the rendered email as its own sandboxed
// document for the console's preview frame. Images are inlined as data URIs
// because the email references them by cid:.
func (a *App) previewCampaignTemplate(w http.ResponseWriter, r *http.Request, p principal, templateID string) {
	t, ok := campaigns.Get(templateID)
	if !ok || !t.Registrants {
		fail(w, 404, "email not found")
		return
	}
	email, e := a.staffEmail(r.Context(), p.ID)
	if e != nil {
		fail(w, 503, "preview unavailable")
		return
	}
	body, _ := t.Render("", email)
	images, e := t.Images()
	if e != nil {
		fail(w, 503, "preview unavailable")
		return
	}
	for _, img := range images {
		body = strings.ReplaceAll(body, "cid:"+img.Name, "data:"+img.ContentType+";base64,"+base64.StdEncoding.EncodeToString(img.Data))
	}
	w.Header().Set("X-Frame-Options", "SAMEORIGIN")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data: https:; style-src 'unsafe-inline'; frame-ancestors 'self'; sandbox allow-popups allow-popups-to-escape-sandbox")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(body))
}

const SamplePassNumber = "TEST-0000"

func passAttachment(number string, pdf []byte) map[string]string {
	return map[string]string{"Name": "Bio-Connect-4.0-pass-" + number + ".pdf", "Content": base64.StdEncoding.EncodeToString(pdf), "ContentType": "application/pdf"}
}

func (a *App) sendCampaignEmail(ctx context.Context, t campaigns.Template, subjectPrefix, email, name string, extra []map[string]string, metadata map[string]string) sendResult {
	if !a.Config.LiveDelivery {
		return sendResult{ID: "fake-campaign-" + id(), Status: "delivered", Code: "fake_provider"}
	}
	images, e := t.Images()
	if e != nil {
		return sendResult{Status: "failed", Code: "template_images"}
	}
	attachments := make([]map[string]string, len(images), len(images)+len(extra))
	for i, img := range images {
		attachments[i] = map[string]string{"Name": img.Name, "Content": base64.StdEncoding.EncodeToString(img.Data), "ContentType": img.ContentType, "ContentID": "cid:" + img.Name}
	}
	attachments = append(attachments, extra...)
	htmlBody, textBody := t.Render(name, email)
	meta := map[string]string{"application": "bioconnect4", "template": t.ID}
	for k, v := range metadata {
		meta[k] = v
	}
	payload := map[string]any{"From": a.Config.SenderName + " <" + a.Config.SenderAddress + ">", "To": email, "ReplyTo": "bioconnect@bio360.in", "Subject": subjectPrefix + t.Subject, "HtmlBody": htmlBody, "TextBody": textBody, "MessageStream": a.Config.PostmarkBroadcastStream, "TrackOpens": false, "TrackLinks": "None", "Metadata": meta, "Attachments": attachments}
	return providerRequest(ctx, a.Config.PostmarkAPIBase+"/email", "X-Postmark-Server-Token", a.Config.PostmarkToken, payload, "email")
}

// CampaignWorker drains the campaign queue one message at a time, separately
// from transactional delivery so a large campaign never delays a pass or OTP.
func (a *App) CampaignWorker(ctx context.Context) {
	for {
		worked, e := a.CampaignWorkOnce(ctx)
		if e != nil {
			slog.Error("campaign worker cycle failed", "error_type", fmt.Sprintf("%T", e))
		}
		if worked && e == nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// CampaignWorkOnce sends at most one queued message and reports whether it did.
func (a *App) CampaignWorkOnce(ctx context.Context) (bool, error) {
	// A crashed send may already have reached Postmark. Never resend it automatically.
	if _, e := a.DB.Exec(ctx, "UPDATE campaign_recipients SET status='uncertain',error_code='worker_lease_expired',updated_at=now() WHERE status='sending' AND claimed_at<now()-interval '5 minutes'"); e != nil {
		return false, e
	}
	var rid, cid, email, name, templateID, passID string
	var attempts int
	e := a.DB.QueryRow(ctx, `UPDATE campaign_recipients cr SET status='sending',attempts=attempts+1,claimed_at=now(),updated_at=now()
		FROM campaigns cp WHERE cp.id=cr.campaign_id AND cr.id=(SELECT id FROM campaign_recipients WHERE status='queued' AND available_at<=now() ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
		RETURNING cr.id,cr.campaign_id,cr.email,cr.name,cr.attempts,cp.template_id,COALESCE(cr.pass_id,'')`).Scan(&rid, &cid, &email, &name, &attempts, &templateID, &passID)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	var result sendResult
	t, ok := campaigns.Get(templateID)
	switch {
	case !ok:
		// An older build still draining the queue during a deploy may not know a
		// new template; back off so the current build sends it.
		result = sendResult{Status: "failed", Code: "template_missing", Retry: true}
	case !t.Open(a.Now()):
		result = sendResult{Status: "cancelled", Code: "template_expired"}
	case t.AttachPass && passID == "":
		result = sendResult{Status: "failed", Code: "pass_missing"}
	default:
		var extra []map[string]string
		if t.AttachPass {
			extra, result = a.campaignPass(ctx, passID)
		}
		if result.Status == "" {
			result = a.sendCampaignEmail(ctx, t, "", email, name, extra, map[string]string{"campaign_id": cid, "campaign_recipient_id": rid})
		}
	}
	backoff := 0
	if result.Retry && attempts < 5 {
		result.Status = "queued"
		backoff = 1 << attempts
	}
	// A webhook can arrive before this update; keep its final status.
	_, e = a.DB.Exec(ctx, `UPDATE campaign_recipients SET provider_id=NULLIF($3,''),error_code=$4,available_at=now()+make_interval(mins=>$5),updated_at=now(),
		status=CASE WHEN $3<>'' AND EXISTS(SELECT 1 FROM webhook_events WHERE channel='email' AND provider_id=$3 AND status='delivered') THEN 'delivered'
		 WHEN $3<>'' AND EXISTS(SELECT 1 FROM webhook_events WHERE channel='email' AND provider_id=$3 AND status='failed') THEN 'failed' ELSE $2 END
		WHERE id=$1`, rid, result.Status, result.ID, result.Code, backoff)
	if e != nil {
		return true, e
	}
	if result.Status == "failed" || result.Status == "uncertain" {
		slog.Error("campaign message needs review", "campaign_id", cid, "recipient_id", rid, "status", result.Status, "code", result.Code)
	}
	return true, nil
}

// campaignPass returns the recipient's current pass as an attachment. A pass
// revoked, or a registration no longer approved, since the snapshot cancels the
// message rather than sending a pass that will not admit them.
func (a *App) campaignPass(ctx context.Context, passID string) ([]map[string]string, sendResult) {
	var number string
	var valid bool
	e := a.DB.QueryRow(ctx, "SELECT p.number,p.revoked_at IS NULL AND r.status='approved' FROM passes p JOIN registrations r ON r.id=p.registration_id WHERE p.id=$1", passID).Scan(&number, &valid)
	if e != nil {
		return nil, sendResult{Status: "failed", Code: "pass_unavailable", Retry: true}
	}
	if !valid {
		return nil, sendResult{Status: "cancelled", Code: "pass_revoked"}
	}
	b, e := a.passPDF(ctx, passID)
	if e != nil {
		return nil, sendResult{Status: "failed", Code: "pdf_unavailable", Retry: true}
	}
	return []map[string]string{passAttachment(number, b)}, sendResult{}
}

// SamplePassPDF renders the pass attached to test emails: never someone's real
// pass, and its QR matches no pass at the venue.
func SamplePassPDF() ([]byte, error) {
	return renderPass("SAMPLE - NOT VALID", "Bio Connect test email", "Sample attendee", "industry", "Industry", "TEST-0000", "sample-not-valid")
}
