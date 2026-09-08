package app

import (
	"fmt"
	"html"
	"strings"
)

// emailHTML wraps a short transactional message in the Bio Connect 4.0 shell:
// a forest header with the logo, a lime rule, the body, and a lime CTA button.
// Table-based, inline styles, hosted logo - the combination email clients tolerate.
func emailHTML(base, heading, intro, ctaLabel, ctaURL string) string {
	esc := html.EscapeString
	body := strings.ReplaceAll(esc(strings.TrimSpace(intro)), "\n", "<br>")
	button, linkLine := "", ""
	if ctaURL != "" {
		button = fmt.Sprintf(`<table role="presentation" cellpadding="0" cellspacing="0" style="margin:26px 0 0;"><tr><td style="border-radius:8px;background:#b9dc72;"><a href="%s" style="display:inline-block;padding:13px 28px;font-family:'Segoe UI',Helvetica,Arial,sans-serif;font-size:13px;font-weight:700;letter-spacing:0.08em;text-transform:uppercase;color:#051c17;text-decoration:none;">%s</a></td></tr></table>`, esc(ctaURL), esc(ctaLabel))
		linkLine = fmt.Sprintf(`<p style="margin:20px 0 0;font-size:12px;line-height:1.6;color:#6b7a72;word-break:break-all;">Or open this link directly:<br><a href="%s" style="color:#0b3329;">%s</a></p>`, esc(ctaURL), esc(ctaURL))
	}
	return fmt.Sprintf(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"></head>`+
		`<body style="margin:0;padding:0;background:#f3f1e9;">`+
		`<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f3f1e9;"><tr><td align="center" style="padding:28px 12px;">`+
		`<table role="presentation" width="560" cellpadding="0" cellspacing="0" style="width:100%%;max-width:560px;background:#ffffff;border:1px solid rgba(16,32,27,0.12);border-radius:14px;overflow:hidden;font-family:'Segoe UI',Helvetica,Arial,sans-serif;color:#10201b;">`+
		`<tr><td style="background:#0b3329;padding:24px 32px;"><span style="display:inline-block;background:#ffffff;border-radius:4px;padding:8px 10px;line-height:0;"><img src="%s/static/bio-connect-logo.png" alt="Bio Connect 4.0" width="140" style="display:block;border:0;width:140px;height:auto;"></span></td></tr>`+
		`<tr><td style="height:3px;background:#b9dc72;font-size:0;line-height:0;">&nbsp;</td></tr>`+
		`<tr><td style="padding:30px 32px;">`+
		`<p style="margin:0 0 6px;font-size:11px;font-weight:700;letter-spacing:0.12em;text-transform:uppercase;color:#4f7f3b;">08&ndash;09 October 2026 &middot; Hyatt Regency Trivandrum</p>`+
		`<h1 style="margin:0 0 14px;font-size:23px;line-height:1.22;color:#10201b;">%s</h1>`+
		`<p style="margin:0;font-size:15px;line-height:1.65;color:#3a4a42;">%s</p>%s%s`+
		`</td></tr>`+
		`<tr><td style="padding:20px 32px;background:#f3f1e9;font-size:12px;line-height:1.7;color:#6b7a72;">Questions? <a href="mailto:bioconnect@bio360.in" style="color:#0b3329;">bioconnect@bio360.in</a><br>Sent via Zinvos for Bio Connect 4.0.</td></tr>`+
		`</table></td></tr></table></body></html>`,
		esc(strings.TrimRight(base, "/")), esc(heading), body, button, linkLine)
}
