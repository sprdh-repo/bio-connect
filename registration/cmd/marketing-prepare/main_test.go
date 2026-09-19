package main

import (
	"context"
	"errors"
	"net"
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
	got := validate(items, nil)
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
	got := validate([]candidate{{Email: "person@gmai.com", Status: "pending"}}, map[string]string{"gmai.com": "gmail.com"})
	if got[0].Status != "eligible" || got[0].Email != "person@gmail.com" || !strings.Contains(got[0].Reason, "confirmed domain correction") {
		t.Fatalf("correction not applied and reported: %#v", got[0])
	}
}
