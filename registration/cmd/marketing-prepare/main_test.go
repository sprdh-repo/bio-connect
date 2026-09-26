package main

import (
	"context"
	"encoding/csv"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanRowsExtractsMultipleAddressesAndReportsMalformedValues(t *testing.T) {
	rows := [][]string{
		{"Company", "E mail"},
		{"One", "First@Example.com\nsecond@example.com"},
		{"Two", "sahil.kalra.resmed.com"},
		{"Three", "broken@@example.com"},
	}
	got := scanRows("contacts.xlsx", "Sheet1", rows, nil)
	if len(got) != 4 {
		t.Fatalf("got %d candidates: %#v", len(got), got)
	}
	if got[0].Email != "first@example.com" || got[1].Email != "second@example.com" {
		t.Fatalf("addresses not normalized: %#v", got)
	}
	if got[2].Status != "invalid_syntax" || !strings.Contains(got[2].Reason, "missing @") {
		t.Fatalf("missing-at value not reported: %#v", got[2])
	}
	if got[3].Status != "invalid_syntax" {
		t.Fatalf("malformed address not reported: %#v", got[3])
	}
}

func TestScanRowsAppliesConfirmedWholeAddressCorrection(t *testing.T) {
	rows := [][]string{{"Email"}, {"mgmt@xopack .com"}}
	got := scanRows("contacts.xlsx", "Sheet1", rows, map[string]string{"mgmt@xopack .com": "mgmt@xopack.com"})
	if len(got) != 1 || got[0].Email != "mgmt@xopack.com" || got[0].Original != "mgmt@xopack .com" || !strings.Contains(got[0].Reason, "confirmed address correction") {
		t.Fatalf("whole-address correction not applied and reported: %#v", got)
	}
}

func TestValidateFiltersDuplicatesTyposNullMXAndMissingDNS(t *testing.T) {
	originalMX, originalHost := lookupMX, lookupHost
	defer func() { lookupMX, lookupHost = originalMX, originalHost }()
	lookupMX = func(_ context.Context, domain string) ([]*net.MX, error) {
		switch domain {
		case "ok.example":
			return []*net.MX{{Host: "mail.ok.example."}}, nil
		case "null.example":
			return []*net.MX{{Host: "."}}, nil
		default:
			return nil, errors.New("not found")
		}
	}
	lookupHost = func(_ context.Context, domain string) ([]string, error) {
		if domain == "fallback.example" {
			return []string{"192.0.2.1"}, nil
		}
		return nil, errors.New("not found")
	}
	items := []candidate{
		{Email: "one@ok.example", Status: "pending"},
		{Email: "one@ok.example", Status: "pending"},
		{Email: "two@gamil.com", Status: "pending"},
		{Email: "three@null.example", Status: "pending"},
		{Email: "four@fallback.example", Status: "pending"},
		{Email: "five@missing.example", Status: "pending"},
	}
	got := validate(items, nil, nil)
	want := []string{"eligible", "duplicate", "suspicious_domain", "unresolvable_domain", "eligible", "unresolvable_domain"}
	for i := range got {
		if got[i].Status != want[i] {
			t.Fatalf("item %d status = %s, want %s (%s)", i, got[i].Status, want[i], got[i].Reason)
		}
	}
}

func TestValidateAppliesOnlyConfirmedDomainCorrection(t *testing.T) {
	originalMX, originalHost := lookupMX, lookupHost
	defer func() { lookupMX, lookupHost = originalMX, originalHost }()
	lookupMX = func(_ context.Context, domain string) ([]*net.MX, error) {
		if domain != "gmail.com" {
			t.Fatalf("looked up uncorrected domain %q", domain)
		}
		return []*net.MX{{Host: "mail.gmail.com."}}, nil
	}
	lookupHost = func(context.Context, string) ([]string, error) { return nil, errors.New("unexpected fallback") }
	got := validate([]candidate{{Email: "person@gmai.com", Status: "pending"}}, map[string]string{"gmai.com": "gmail.com"}, nil)
	if got[0].Status != "eligible" || got[0].Email != "person@gmail.com" || !strings.Contains(got[0].Reason, "confirmed domain correction") {
		t.Fatalf("correction not applied and reported: %#v", got[0])
	}
}

func TestHistoryEmailsAndValidationExcludeEveryPriorAttempt(t *testing.T) {
	originalMX, originalHost := lookupMX, lookupHost
	defer func() { lookupMX, lookupHost = originalMX, originalHost }()
	lookupMX = func(context.Context, string) ([]*net.MX, error) {
		return []*net.MX{{Host: "mail.example.com."}}, nil
	}
	lookupHost = func(context.Context, string) ([]string, error) {
		return nil, errors.New("unexpected fallback")
	}

	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "sends.jsonl")
	jsonData := "{\"Email\":\"Attempted@Example.com\",\"Status\":\"attempted\"}\n" +
		"{\"Email\":\"accepted@example.com\",\"Status\":\"accepted\"}\n"
	if err := os.WriteFile(jsonPath, []byte(jsonData), 0600); err != nil {
		t.Fatal(err)
	}
	csvPath := filepath.Join(dir, "report.csv")
	f, err := os.Create(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"campaign", "email", "status"})
	_ = w.Write([]string{"campaign", "uncertain@example.com", "uncertain"})
	_ = w.Write([]string{"campaign", "skipped@example.com", "skipped_previous_accepted"})
	_ = w.Write([]string{"campaign", "fresh@example.com", "not_attempted"})
	w.Flush()
	if err := errors.Join(w.Error(), f.Close()); err != nil {
		t.Fatal(err)
	}

	previous := map[string]bool{}
	for _, path := range []string{jsonPath, csvPath} {
		found, err := historyEmails(path)
		if err != nil {
			t.Fatal(err)
		}
		for email := range found {
			previous[email] = true
		}
	}
	for _, email := range []string{"attempted@example.com", "accepted@example.com", "uncertain@example.com", "skipped@example.com"} {
		if !previous[email] {
			t.Errorf("prior attempt %s was not loaded", email)
		}
	}
	if previous["fresh@example.com"] {
		t.Fatal("not-attempted recipient was excluded")
	}

	items := validate([]candidate{
		{Email: "attempted@example.com", Status: "pending"},
		{Email: "fresh@example.com", Status: "pending"},
	}, nil, previous)
	if items[0].Status != "previously_attempted" || items[1].Status != "eligible" {
		t.Fatalf("unexpected history filtering: %#v", items)
	}
}
