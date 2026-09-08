package app

import (
	"context"
	"encoding/base64"
	"os"
	"testing"
)

// TestLiveSampleEmail sends the real branded Bio Connect 4.0 emails (registration
// saved + pass ready, with a freshly rendered pass PDF attached) through Postmark
// to LIVE_SAMPLE_EMAIL, for a human to review the finished product in an inbox.
//
// It is never part of the normal suite. Run explicitly, e.g.:
//
//	set -a; . ops/secrets/bioconnect-infra.env; set +a
//	LIVE_SAMPLE_EMAIL=you@example.com go test -count=1 -run TestLiveSampleEmail ./internal/app/
func TestLiveSampleEmail(t *testing.T) {
	to := os.Getenv("LIVE_SAMPLE_EMAIL")
	if to == "" {
		t.Skip("set LIVE_SAMPLE_EMAIL to send live sample emails")
	}
	c := FromEnv()
	if c.PostmarkToken == "" || c.SenderAddress == "" {
		t.Fatal("Postmark settings not in environment; source ops/secrets/bioconnect-infra.env")
	}
	ctx := context.Background()
	base := c.BaseURL
	from := c.SenderName + " <" + c.SenderAddress + ">"
	stream := c.PostmarkStream
	if stream == "" {
		stream = "outbound"
	}

	pdf, err := renderPass("Asha Nair", "Kerala University", "PhD Scholar",
		"student", "Students", "BC4-ST-0007-1", "opaque-qr-sample-review")
	if err != nil {
		t.Fatalf("renderPass: %v", err)
	}

	messages := []struct {
		subject, heading, intro, cta, link string
		attach                             bool
	}{
		{
			subject: "Bio Connect 4.0 - registration saved",
			heading: "Registration saved",
			intro:   "Your registration is saved. Use this private link to see the payment instructions and submit your evidence.\n\nThis is not a payment approval or an admission pass - keep the link to yourself.",
			cta:     "Open my registration",
			link:    base + "/manage/sample-review-link",
		},
		{
			subject: "Bio Connect 4.0 - your pass",
			heading: "Your pass is ready",
			intro:   "Your Bio Connect 4.0 pass is attached to this email. The event takes place on 8-9 October 2026 at Hyatt Regency Trivandrum. You can also open it any time from the link below.\n\nPass number: BC4-ST-0007-1 - quote this if you contact us about your pass.",
			cta:     "View your pass",
			link:    base + "/passes/sample-review-link",
			attach:  true,
		},
	}

	for _, m := range messages {
		payload := map[string]any{
			"From":          from,
			"To":            to,
			"ReplyTo":       "bioconnect@bio360.in",
			"Subject":       m.subject,
			"HtmlBody":      emailHTML(base, m.heading, m.intro, m.cta, m.link),
			"TextBody":      m.intro + "\n\n" + m.link + "\n\nQuestions: bioconnect@bio360.in\nSent via Zinvos for Bio Connect 4.0.",
			"MessageStream": stream,
			"Metadata":      map[string]string{"application": "bioconnect4", "delivery_id": "live-sample"},
		}
		if m.attach {
			payload["Attachments"] = []map[string]string{{
				"Name":        "Bio-Connect-4.0-pass.pdf",
				"Content":     base64.StdEncoding.EncodeToString(pdf),
				"ContentType": "application/pdf",
			}}
		}
		r := providerRequest(ctx, c.PostmarkAPIBase+"/email", "X-Postmark-Server-Token", c.PostmarkToken, payload, "email")
		if r.Status != "accepted" {
			t.Fatalf("%q: status=%s code=%s", m.subject, r.Status, r.Code)
		}
		t.Logf("sent %q to %s (provider id %s)", m.subject, to, r.ID)
	}
}

// TestLiveSampleWhatsApp sends the approved bioconnect_pass_delivery template to
// LIVE_SAMPLE_WHATSAPP (E.164, e.g. +9198...), for a human to review the WhatsApp
// pass-delivery message on a real handset. Never part of the normal suite.
func TestLiveSampleWhatsApp(t *testing.T) {
	to := os.Getenv("LIVE_SAMPLE_WHATSAPP")
	if to == "" {
		t.Skip("set LIVE_SAMPLE_WHATSAPP to send a live sample WhatsApp message")
	}
	c := FromEnv()
	if c.MetaToken == "" || c.MetaPhoneID == "" || c.MetaTemplate == "" {
		t.Fatal("Meta settings not in environment; source ops/secrets/bioconnect-infra.env")
	}
	ctx := context.Background()
	link := c.BaseURL + "/passes/sample-review-link"

	pdf, err := renderPass("Asha Nair", "Kerala University", "PhD Scholar",
		"student", "Students", "BC4-ST-0007-1", "opaque-qr-sample-review")
	if err != nil {
		t.Fatalf("renderPass: %v", err)
	}
	docID, err := uploadWhatsAppMedia(ctx, c, "Bio-Connect-4.0-pass.pdf", pdf)
	if err != nil {
		t.Fatalf("uploadWhatsAppMedia: %v", err)
	}

	payload := map[string]any{
		"messaging_product":        "whatsapp",
		"to":                       to[1:], // Graph API wants the number without the leading +
		"type":                     "template",
		"biz_opaque_callback_data": "live-sample",
		"template": map[string]any{
			"name":     c.MetaTemplate,
			"language": map[string]string{"code": c.MetaLanguage},
			"components": []any{
				map[string]any{
					"type": "header",
					"parameters": []any{map[string]any{
						"type":     "document",
						"document": map[string]string{"id": docID, "filename": "Bio-Connect-4.0-pass.pdf"},
					}},
				},
				map[string]any{
					"type":       "body",
					"parameters": []any{map[string]string{"type": "text", "text": link}},
				},
			},
		},
	}
	url := c.MetaAPIBase + "/" + c.MetaVersion + "/" + c.MetaPhoneID + "/messages"
	r := providerRequest(ctx, url, "Authorization", "Bearer "+c.MetaToken, payload, "whatsapp")
	if r.Status != "accepted" {
		t.Fatalf("whatsapp send: status=%s code=%s", r.Status, r.Code)
	}
	t.Logf("sent template %q to %s (provider id %s)", c.MetaTemplate, to, r.ID)
}
