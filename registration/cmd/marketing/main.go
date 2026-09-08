// Command marketing previews and sends the Bio Connect registration invitation.
package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xuri/excelize/v2"
)

//go:embed invitation.html
var invitation string

//go:embed invitation.txt
var plain string

const subject = "Bio Connect 4.0: register now for Kerala's life sciences summit"

type contact struct{ Email, Name string }
type entry struct {
	Key string
	Status string
	MessageID string
	Time time.Time
}

func address(s string) (string, error) {
	s = strings.TrimSpace(s)
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || !strings.Contains(s, ".") || strings.ContainsAny(s, "\r\n") {
		return "", fmt.Errorf("invalid email address %q", s)
	}
	return strings.ToLower(s), nil
}

func contacts(path, sheet, emailColumn, nameColumn string) ([]contact, error) {
	var rows [][]string
	var err error
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		f, e := os.Open(path); if e != nil { return nil, e }; defer f.Close()
		r := csv.NewReader(f); r.FieldsPerRecord = -1
		rows, err = r.ReadAll()
	case ".xlsx":
		f, e := excelize.OpenFile(path); if e != nil { return nil, e }; defer f.Close()
		if sheet == "" { sheet = f.GetSheetName(0) }
		rows, err = f.GetRows(sheet)
	default: return nil, errors.New("contacts must be .xlsx or .csv; save legacy .xls as .xlsx first")
	}
	if err != nil { return nil, err }; if len(rows) < 2 { return nil, errors.New("contacts file needs a header and at least one contact") }
	column := func(name string) (int, error) {
		idx := -1
		for i, h := range rows[0] {
			if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")), name) {
				if idx >= 0 { return -1, fmt.Errorf("duplicate column %q", name) }; idx = i
			}
		}
		return idx, nil
	}
	ei, err := column(emailColumn); if err != nil { return nil, err }; if ei < 0 { return nil, fmt.Errorf("missing email column %q", emailColumn) }
	ni, err := column(nameColumn); if err != nil { return nil, err }
	if nameColumn != "" && ni < 0 { return nil, fmt.Errorf("missing name column %q", nameColumn) }
	seen := map[string]bool{}; result := []contact{}
	for i, row := range rows[1:] {
		if strings.TrimSpace(strings.Join(row, "")) == "" { continue }
		if ei >= len(row) { return nil, fmt.Errorf("row %d: missing email", i+2) }
		email, e := address(row[ei]); if e != nil { return nil, fmt.Errorf("row %d: %w", i+2, e) }
		if seen[email] { continue }; seen[email] = true
		name := ""; if ni >= 0 && ni < len(row) { name = strings.TrimSpace(row[ni]) }
		result = append(result, contact{email, name})
	}
	if len(result) == 0 { return nil, errors.New("no contacts") }; return result, nil
}

func render(c contact) (string, string) {
	greeting := "Hello,"; if c.Name != "" { greeting = "Hello " + c.Name + "," }
	return strings.ReplaceAll(invitation, "__GREETING__", html.EscapeString(greeting)), strings.ReplaceAll(plain, "__GREETING__", greeting)
}

type provider struct { client *http.Client; base, token string }
func (p provider) request(method, path string, payload, result any) error {
	var body io.Reader
	if payload != nil { b, err := json.Marshal(payload); if err != nil { return err }; body = bytes.NewReader(b) }
	req, err := http.NewRequest(method, p.base+path, body); if err != nil { return err }
	req.Header.Set("X-Postmark-Server-Token", p.token)
	req.Header.Set("Content-Type", "application/json"); req.Header.Set("Accept", "application/json")
	resp, err := p.client.Do(req); if err != nil { return err }; defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return fmt.Errorf("Postmark HTTP %d; check Postmark activity before retrying", resp.StatusCode) }
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(result)
}

func main() { if err := run(); err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) } }
func run() error {
	file := flag.String("contacts", "", "Excel (.xlsx) or CSV contacts")
	sheet := flag.String("sheet", "", "Excel sheet (default: first)")
	emailCol := flag.String("email-column", "Email", "email header")
	nameCol := flag.String("name-column", "", "optional name header")
	test := flag.String("test-to", "", "send one test, ignoring contact files")
	send := flag.Bool("send", false, "send the reviewed campaign to contacts")
	campaign := flag.String("campaign", "registration-september-2026", "stable campaign ID for duplicate prevention")
	state := flag.String("state", "var/marketing", "private send-log and preview directory")
	stream := flag.String("stream", "broadcast", "Postmark broadcast stream with Postmark-managed unsubscribes")
	flag.Parse()
	if flag.NArg() > 0 { return errors.New("unexpected positional arguments") }
	if *test != "" && (*send || *file != "") { return errors.New("test mode cannot be combined with contacts or --send") }
	if *send && *file == "" { return errors.New("--send requires --contacts") }
	if strings.TrimSpace(*campaign) == "" { return errors.New("campaign must not be empty") }
	list := []contact{{Name: "Shiyaf"}}
	var err error
	if *file != "" { list, err = contacts(*file, *sheet, *emailCol, *nameCol); if err != nil { return err } }
	if *test != "" { list[0].Email, err = address(*test); if err != nil { return err } }
	if err := os.MkdirAll(*state, 0700); err != nil { return err }
	h, t := render(list[0])
	for ext, content := range map[string]string{"html":h, "txt":t} {
		if err := os.WriteFile(filepath.Join(*state, "preview."+ext), []byte(content), 0600); err != nil { return err }
	}
	fmt.Printf("Preview: %s/preview.html\n", *state)
	if !*send && *test == "" { fmt.Printf("Dry run: %d unique contacts; no emails sent.\n", len(list)); return nil }
	// This invitation is deliberately dated; require a copy update after the offer ends.
	loc := time.FixedZone("IST", 19800)
	if !time.Now().Before(time.Date(2026,10,1,0,0,0,0,loc)) { return errors.New("early-bird invitation expired; update the copy and cutoff before sending") }
	token := os.Getenv("POSTMARK_SERVER_TOKEN")
	from, err := address(os.Getenv("POSTMARK_FROM_ADDRESS")); if err != nil { return fmt.Errorf("POSTMARK_FROM_ADDRESS: %w", err) }
	if token == "" || token == "POSTMARK_API_TEST" { return errors.New("a real POSTMARK_SERVER_TOKEN is required") }
	p := provider{&http.Client{Timeout:30*time.Second, CheckRedirect:func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, "https://api.postmarkapp.com", token}
	var info struct { MessageStreamType string; ArchivedAt *string; SubscriptionManagementConfiguration struct { UnsubscribeHandlingType string } }
	if err := p.request("GET", "/message-streams/"+url.PathEscape(*stream), nil, &info); err != nil { return err }
	if info.MessageStreamType != "Broadcasts" || info.ArchivedAt != nil || info.SubscriptionManagementConfiguration.UnsubscribeHandlingType != "Postmark" { return errors.New("stream must be active Broadcasts with Postmark unsubscribe handling") }
	log, err := os.OpenFile(filepath.Join(*state,"sends.jsonl"), os.O_CREATE|os.O_RDWR|os.O_APPEND,0600); if err != nil { return err }; defer log.Close()
	if err := syscall.Flock(int(log.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { return errors.New("another sender is using this state directory") }
	defer syscall.Flock(int(log.Fd()), syscall.LOCK_UN)
	seen := map[string]bool{}; dec := json.NewDecoder(log)
	for { var e entry; err := dec.Decode(&e); if err == io.EOF { break }; if err != nil { return fmt.Errorf("damaged send log: %w",err) }; seen[e.Key] = true }
	appendEntry := func(e entry) error { if err := json.NewEncoder(log).Encode(e); err != nil { return err }; return log.Sync() }
	name := os.Getenv("POSTMARK_FROM_NAME"); if name == "" { name = "Bio Connect 4.0" }
	for _, c := range list {
		key := fmt.Sprintf("%x",sha256.Sum256([]byte(*stream+"\x00"+*campaign+"\x00"+c.Email)))
		if *test != "" { key += ":test:"+time.Now().UTC().Format(time.RFC3339Nano) }
		if seen[key] { fmt.Printf("Skipped previously attempted contact: %s\n",c.Email); continue }
		h, t := render(c); sub := subject; if *test != "" { sub = "[TEST] "+sub }
		payload := map[string]any{"From":(&mail.Address{Name:name,Address:from}).String(),"To":c.Email,"ReplyTo":"bioconnect@bio360.in","Subject":sub,"HtmlBody":h,"TextBody":t,"MessageStream":*stream,"TrackOpens":false,"TrackLinks":"None","Metadata":map[string]string{"campaign":*campaign,"application":"bioconnect-marketing"}}
		// Persist before the request. An interrupted/ambiguous attempt is never retried automatically.
		if err := appendEntry(entry{key,"attempted","",time.Now().UTC()}); err != nil { return err }
		var result struct { ErrorCode int; MessageID, Message string }
		if err := p.request("POST","/email",payload,&result); err != nil { return fmt.Errorf("send to %s uncertain: %w; attempt retained, no automatic retry",c.Email,err) }
		if result.ErrorCode != 0 || result.MessageID == "" { return fmt.Errorf("Postmark rejected %s (code %d): %s; attempt retained",c.Email,result.ErrorCode,result.Message) }
		if err := appendEntry(entry{key,"accepted",result.MessageID,time.Now().UTC()}); err != nil { return fmt.Errorf("Postmark accepted %s but logging failed: %w",result.MessageID,err) }
		fmt.Printf("Accepted: %s (MessageID %s)\n",c.Email,result.MessageID)
	}
	return nil
}
