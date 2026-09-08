package app

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

var repeatableRead = pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}

func (a *App) export(w http.ResponseWriter, r *http.Request, p principal) {
	where, args, e := filter(r)
	if e != nil {
		fail(w, 400, e.Error())
		return
	}
	queries := map[string]string{
		"Registrations": `SELECT r.reference,r.category_id,r.institution,r.contact_name,r.email,r.phone,r.description,r.status,r.quoted_paise,r.created_at,r.approved_at,s.email AS approved_by,r.review_note,(SELECT string_agg(DISTINCT d.channel||':'||d.status,', ' ORDER BY d.channel||':'||d.status) FROM delivery_jobs d WHERE d.registration_id=r.id) AS delivery_summary FROM registrations r LEFT JOIN staff s ON s.id=r.approved_by`,
		"Attendees":     `SELECT r.reference,r.institution,r.category_id,r.status,a.name,a.designation,a.email,a.phone,a.whatsapp_consent,a.consent_at,a.consent_text,p.number AS pass_number,p.version,p.revoked_at,r.approved_at,(SELECT string_agg(DISTINCT d.channel||':'||d.status,', ' ORDER BY d.channel||':'||d.status) FROM delivery_jobs d WHERE d.pass_id=p.id) AS delivery_summary FROM registrations r JOIN attendees a ON a.registration_id=r.id LEFT JOIN passes p ON p.attendee_id=a.id AND p.revoked_at IS NULL`,
		"Payments":      `SELECT r.reference,r.status,p.bank_reference,p.payment_date,p.amount_paise,p.receipt_id,p.created_at,p.verified_reference,p.verified_date,p.verified_amount_paise,p.verified_at,s.email AS verified_by,p.beneficiary_confirmed FROM registrations r JOIN payment_submissions p ON p.registration_id=r.id LEFT JOIN staff s ON s.id=p.verified_by`,
		"Deliveries":    `SELECT r.reference,d.id,d.pass_id,d.purpose,d.channel,d.recipient,d.status,d.attempts,d.provider_id,d.error_code,d.created_at,d.updated_at FROM registrations r JOIN delivery_jobs d ON d.registration_id=r.id`,
	}
	names := []string{"Registrations", "Attendees", "Payments", "Deliveries"}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "xlsx"
	}
	if format != "csv" && format != "xlsx" {
		fail(w, 400, "format must be csv or xlsx")
		return
	}
	if format == "csv" {
		sheet := r.URL.Query().Get("sheet")
		if sheet == "" {
			sheet = "Registrations"
		}
		if queries[sheet] == "" {
			fail(w, 400, "unknown export sheet")
			return
		}
		names = []string{sheet}
	}
	// A repeatable-read snapshot keeps every sheet consistent with the same set of approvals.
	tx, e := a.DB.BeginTx(r.Context(), repeatableRead)
	if e != nil {
		fail(w, 503, "export unavailable")
		return
	}
	defer tx.Rollback(r.Context())
	book := excelize.NewFile()
	defer book.Close()
	var csvOut bytes.Buffer
	for i, name := range names {
		rows, e := tx.Query(r.Context(), queries[name]+where+" ORDER BY r.created_at,r.id LIMIT 50001", args...)
		if e != nil {
			fail(w, 503, "export unavailable")
			return
		}
		header := []string{}
		for _, f := range rows.FieldDescriptions() {
			header = append(header, f.Name)
		}
		data := [][]string{header}
		for rows.Next() {
			vals, e := rows.Values()
			if e != nil {
				rows.Close()
				fail(w, 503, "export unavailable")
				return
			}
			row := make([]string, len(vals))
			for j, v := range vals {
				switch val := v.(type) {
				case nil:
					row[j] = ""
				case time.Time:
					row[j] = val.In(india).Format(time.RFC3339)
				default:
					row[j] = safeCell(fmt.Sprint(v))
				}
			}
			data = append(data, row)
			if len(data) > 50001 {
				rows.Close()
				fail(w, 400, "export exceeds 50,000 rows; narrow the date filters")
				return
			}
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			fail(w, 503, "export unavailable")
			return
		}
		if format == "csv" {
			cw := csv.NewWriter(&csvOut)
			cw.WriteAll(data)
			if e = cw.Error(); e != nil {
				fail(w, 500, "export failed")
				return
			}
		} else {
			if i == 0 {
				e = book.SetSheetName("Sheet1", name)
			} else {
				_, e = book.NewSheet(name)
			}
			if e != nil {
				fail(w, 500, "export failed")
				return
			}
			for ri, row := range data {
				for ci, value := range row {
					cell, _ := excelize.CoordinatesToCellName(ci+1, ri+1)
					if e = book.SetCellStr(name, cell, value); e != nil {
						fail(w, 500, "export failed")
						return
					}
				}
			}
			last, _ := excelize.ColumnNumberToName(len(header))
			_ = book.SetColWidth(name, "A", last, 23)
			_ = book.SetPanes(name, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
			style, _ := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"0B3329"}, Pattern: 1}})
			_ = book.SetCellStyle(name, "A1", last+"1", style)
		}
	}
	if e = tx.Commit(r.Context()); e != nil {
		fail(w, 503, "export unavailable")
		return
	}
	if _, e = a.DB.Exec(r.Context(), "INSERT INTO audit_events(staff_id,action,detail) VALUES($1,'export',$2)", p.ID, format); e != nil {
		fail(w, 503, "export audit unavailable")
		return
	}
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="Bio-Connect-4.0-`+names[0]+`.csv"`)
		w.Write(csvOut.Bytes())
		return
	}
	b, e := book.WriteToBuffer()
	if e != nil {
		fail(w, 500, "export failed")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="Bio-Connect-4.0.xlsx"`)
	w.Write(b.Bytes())
}
