// Command marketing-prepare extracts and validates email addresses from messy contact workbooks.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/xuri/excelize/v2"
)

var (
	lookupMX   = net.DefaultResolver.LookupMX
	lookupHost = net.DefaultResolver.LookupHost
	emailRE    = regexp.MustCompile("(?i)[a-z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+")
)

type candidate struct {
	Source, Sheet, Cell, Original, Email, Status, Reason string
}

type stringFlags []string

func (values *stringFlags) String() string { return strings.Join(*values, ",") }
func (values *stringFlags) Set(value string) error {
	*values = append(*values, value)
	return nil
}

var commonDomainTypos = map[string]string{
	"gmai.com": "gmail.com", "gamil.com": "gmail.com", "gmail.co": "gmail.com", "gmail.con": "gmail.com", "gmial.com": "gmail.com",
	"hotmal.com": "hotmail.com", "hotmai.com": "hotmail.com", "outlok.com": "outlook.com",
	"yaho.com": "yahoo.com", "yahooo.com": "yahoo.com", "rediffmal.com": "rediffmail.com",
}

func normalHeader(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.NewReplacer("_", " ", "-", " ", ".", "").Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func isEmailHeader(s string) bool {
	switch normalHeader(s) {
	case "email", "e mail", "email id", "e mail id", "mail", "mail id", "email address", "mail address":
		return true
	}
	return false
}

func strictAddress(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || strings.ContainsAny(s, "\r\n") {
		return "", fmt.Errorf("invalid email syntax")
	}
	return s, nil
}

func domainOf(email string) string { return email[strings.LastIndex(email, "@")+1:] }

func appendReason(current, next string) string {
	if current == "" {
		return next
	}
	return current + "; " + next
}

func looksLikeMissingAt(s string) bool {
	s = strings.TrimSpace(s)
	return !strings.ContainsAny(s, " @\t\r\n") && strings.Count(s, ".") >= 2
}

func scanRows(source, sheet string, rows [][]string, addressCorrections map[string]string) []candidate {
	var result []candidate
	emailColumns := map[int]bool{}
	for rowIndex, row := range rows {
		var headers []int
		for column, value := range row {
			if isEmailHeader(value) {
				headers = append(headers, column)
			}
		}
		if len(headers) > 0 {
			emailColumns = map[int]bool{}
			for _, column := range headers {
				emailColumns[column] = true
			}
			continue
		}
		for column, value := range row {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			cell, _ := excelize.CoordinatesToCellName(column+1, rowIndex+1)
			original := value
			correctionReason := ""
			if corrected, ok := addressCorrections[strings.ToLower(value)]; ok {
				value = corrected
				correctionReason = "confirmed address correction: " + original + " to " + corrected
			}
			matches := emailRE.FindAllString(value, -1)
			for _, match := range matches {
				email, err := strictAddress(match)
				item := candidate{Source: source, Sheet: sheet, Cell: cell, Original: original, Email: email, Status: "pending", Reason: correctionReason}
				if err != nil {
					item.Status, item.Reason = "invalid_syntax", err.Error()
				}
				result = append(result, item)
			}
			if strings.Contains(value, "@") && len(matches) == 0 {
				result = append(result, candidate{Source: source, Sheet: sheet, Cell: cell, Original: value, Status: "invalid_syntax", Reason: "contains @ but is not a valid email address"})
			} else if emailColumns[column] && len(matches) == 0 && looksLikeMissingAt(value) {
				result = append(result, candidate{Source: source, Sheet: sheet, Cell: cell, Original: value, Status: "invalid_syntax", Reason: "email-like value is missing @"})
			}
		}
	}
	return result
}

func scanFile(path string, addressCorrections map[string]string) ([]candidate, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".xlsx":
		f, err := excelize.OpenFile(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var result []candidate
		for _, sheet := range f.GetSheetList() {
			rows, err := f.GetRows(sheet)
			if err != nil {
				return nil, err
			}
			result = append(result, scanRows(path, sheet, rows, addressCorrections)...)
		}
		return result, nil
	case ".csv":
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r := csv.NewReader(f)
		r.FieldsPerRecord = -1
		rows, err := r.ReadAll()
		if err != nil {
			return nil, err
		}
		return scanRows(path, "CSV", rows, addressCorrections), nil
	default:
		return nil, errors.New("contacts must be .xlsx or .csv")
	}
}

func checkDomain(ctx context.Context, domain string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	mxs, mxErr := lookupMX(ctx, domain)
	for _, mx := range mxs {
		if strings.TrimSuffix(mx.Host, ".") != "" {
			return nil
		}
	}
	if mxErr == nil && len(mxs) > 0 {
		return fmt.Errorf("domain publishes a null MX and does not accept email")
	}
	if _, err := lookupHost(ctx, domain); err != nil {
		return fmt.Errorf("no MX or A/AAAA mail fallback: %v", err)
	}
	return nil
}

func validate(items []candidate, corrections map[string]string) []candidate {
	seen := map[string]bool{}
	domains := map[string]bool{}
	for i := range items {
		if items[i].Status != "pending" {
			continue
		}
		domain := domainOf(items[i].Email)
		if corrected, ok := corrections[domain]; ok {
			items[i].Email = items[i].Email[:strings.LastIndex(items[i].Email, "@")+1] + corrected
			items[i].Reason = "confirmed domain correction: " + domain + " to " + corrected
		}
		if seen[items[i].Email] {
			items[i].Status = "duplicate"
			items[i].Reason = appendReason(items[i].Reason, "same normalized address already appears earlier")
			continue
		}
		seen[items[i].Email] = true
		domain = domainOf(items[i].Email)
		if expected, ok := commonDomainTypos[domain]; ok {
			items[i].Status, items[i].Reason = "suspicious_domain", "possible typo; expected "+expected
			continue
		}
		domains[domain] = true
	}
	type domainResult struct {
		domain string
		err    error
	}
	results := make(chan domainResult, len(domains))
	sem := make(chan struct{}, 20)
	var wg sync.WaitGroup
	for domain := range domains {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results <- domainResult{domain, checkDomain(context.Background(), domain)}
		}()
	}
	wg.Wait()
	close(results)
	domainErrors := map[string]error{}
	for result := range results {
		if result.err != nil {
			domainErrors[result.domain] = result.err
		}
	}
	for i := range items {
		if items[i].Status != "pending" {
			continue
		}
		if err := domainErrors[domainOf(items[i].Email)]; err != nil {
			items[i].Status = "unresolvable_domain"
			items[i].Reason = appendReason(items[i].Reason, err.Error())
		} else {
			items[i].Status = "eligible"
		}
	}
	return items
}

func safeCSVCell(s string) string {
	trimmed := strings.TrimSpace(s)
	if trimmed != "" && strings.ContainsAny(trimmed[:1], "=+-@\t\r") {
		return "'" + s
	}
	return s
}

func writeValidation(path string, items []candidate) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"source_file", "sheet", "cell", "original_value", "normalized_email", "status", "reason"})
	for _, item := range items {
		row := []string{item.Source, item.Sheet, item.Cell, item.Original, item.Email, item.Status, item.Reason}
		for i := range row {
			row[i] = safeCSVCell(row[i])
		}
		_ = w.Write(row)
	}
	w.Flush()
	return w.Error()
}

func writeContacts(path string, items []candidate) (int, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"Email"})
	count := 0
	for _, item := range items {
		if item.Status == "eligible" {
			_ = w.Write([]string{item.Email})
			count++
		}
	}
	w.Flush()
	return count, w.Error()
}

func run() error {
	output := flag.String("output", "var/marketing/validated-contacts.csv", "filtered contacts CSV")
	report := flag.String("report", "var/marketing/validation.csv", "complete validation report CSV")
	var domainCorrectionFlags, addressCorrectionFlags stringFlags
	flag.Var(&domainCorrectionFlags, "correct-domain", "confirmed domain correction in typo=correct form; repeatable")
	flag.Var(&addressCorrectionFlags, "correct-address", "confirmed whole-address correction in typo=correct form; repeatable")
	flag.Parse()
	if flag.NArg() == 0 {
		return errors.New("provide one or more .xlsx or .csv contact files")
	}
	corrections := map[string]string{}
	for _, correction := range domainCorrectionFlags {
		parts := strings.Split(correction, "=")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(correction, "@, ") {
			return errors.New("--correct-domain must use typo=correct domain form")
		}
		corrections[strings.ToLower(parts[0])] = strings.ToLower(parts[1])
	}
	addressCorrections := map[string]string{}
	for _, correction := range addressCorrectionFlags {
		parts := strings.SplitN(correction, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return errors.New("--correct-address must use typo=correct address form")
		}
		corrected, err := strictAddress(parts[1])
		if err != nil {
			return fmt.Errorf("--correct-address replacement: %w", err)
		}
		addressCorrections[strings.ToLower(strings.TrimSpace(parts[0]))] = corrected
	}
	var items []candidate
	for _, path := range flag.Args() {
		found, err := scanFile(path, addressCorrections)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		items = append(items, found...)
	}
	items = validate(items, corrections)
	for _, path := range []string{*output, *report} {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
	}
	if err := writeValidation(*report, items); err != nil {
		return err
	}
	eligible, err := writeContacts(*output, items)
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, item := range items {
		counts[item.Status]++
	}
	fmt.Printf("Validation report: %s\nFiltered contacts: %s\n", *report, *output)
	fmt.Printf("Eligible: %d; duplicates: %d; invalid syntax: %d; suspicious domains: %d; unresolvable domains: %d\n", eligible, counts["duplicate"], counts["invalid_syntax"], counts["suspicious_domain"], counts["unresolvable_domain"])
	if eligible == 0 {
		return errors.New("no eligible email addresses remain after validation")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
