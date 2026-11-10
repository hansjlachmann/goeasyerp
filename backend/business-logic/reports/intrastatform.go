package reports

import (
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Report 501 Intrastat - Form (Print › Form on the Intrastat Journal 311): the declaration of
// one type — the Type filter must be Receipt or Shipment — as printed lines, one per group of
// the batch's lines with the same country, tariff number, transaction type and transport method
// (classic GroupTotalFields Transport Method), with the group's weight, quantity and
// statistical value; the batch's totals and the number of lines declared
const IntrastatFormID = 501

func init() {
	Register(IntrastatFormID, "intrastat-form", func() Report { return &intrastatForm{} })
}

type intrastatForm struct {
	noOfRecords int
	groupKey    string
}

func (r *intrastatForm) Definition() *Definition {
	return &Definition{
		ContextTable: gtables.IntrastatJnlLineTableName,
		OnInitReport: func(run *Run) error {
			intraJnlBatch, ok := intrastatBatchOf(run)
			if ok {
				run.SetRequestFilter("IntrastatJnlBatch", "journal_template_name", intraJnlBatch.Journal_template_name.String())
				run.SetRequestFilter("IntrastatJnlBatch", "name", intraJnlBatch.Name.String())
			}
			run.SetRequestFilter("IntrastatJnlLine", "type", strconv.Itoa(int(gtables.IntrastatJnlLine_Type.Receipt)))
			return nil
		},
		OnPreReport: func(run *Run) error {
			setReportHeadValues(run)
			companyInfo := gtables.GetCompanyInformation(run)
			run.SetValue("vat_registration_no", system.ConvertStr(companyInfo.Vat_registration_no.String(), "WwWw", "    "))
			return nil
		},
		DataItems: []*DataItem{{
			Name:                "IntrastatJnlBatch",
			Table:               gtables.IntrastatJnlBatchTableName,
			Key:                 []string{"journal_template_name", "name"},
			RequestFilterFields: []string{"journal_template_name", "name"},
			OnAfterGetRecord: Trigger(func(rec *gtables.IntrastatJnlBatch, run *Run) error {
				run.SetPageNo(1)
				r.noOfRecords = 0
				r.groupKey = ""
				run.SetValue("no_of_entries", types.Decimal{})
				run.SetValue("statistics_period", run.Text("STATISTICS_PERIOD", rec.Statistics_period.String()))
				glSetup := gtables.GetGeneralLedgerSetup(run)
				if rec.Amounts_in_add_currency {
					err := glSetup.TestField("additional_reporting_currency")
					if err != nil {
						return err
					}
					run.SetValue("amounts_in", run.Text("AMOUNTS_IN", glSetup.Additional_reporting_currency))
				} else {
					err := glSetup.TestField("lcy_code")
					if err != nil {
						return err
					}
					run.SetValue("amounts_in", run.Text("AMOUNTS_IN", glSetup.Lcy_code))
				}
				return nil
			}),
			DataItems: []*DataItem{{
				Name:                "IntrastatJnlLine",
				Table:               gtables.IntrastatJnlLineTableName,
				Key:                 []string{"type", "country_code", "tariff_no", "transaction_type", "transport_method"},
				Link:                map[string]string{"journal_template_name": "journal_template_name", "journal_batch_name": "name"},
				RequestFilterFields: []string{"type"},
				GroupFields:         []string{"type", "country_code", "tariff_no", "transaction_type", "transport_method"},
				OnPreDataItem:       Trigger(r.lineOnPreDataItem),
				OnAfterGetRecord:    Trigger(r.lineOnAfterGetRecord),
			}},
		}},
	}
}

// lineOnPreDataItem (OnPreReport): the Type filter is one type, Receipt or Shipment
func (r *intrastatForm) lineOnPreDataItem(rec *gtables.IntrastatJnlLine, run *Run) error {
	typeFilter := strings.TrimSpace(rec.GetFilter("type"))
	if typeFilter == "" || strings.ContainsAny(typeFilter, "|.<>*&") {
		return apperrors.FieldErrorText(gtables.IntrastatJnlLineTableName, "type", run.Text("TYPE_ONE"))
	}
	lineType, err := strconv.Atoi(typeFilter)
	if err != nil {
		lineType = -1
		for _, t := range []gtables.IntrastatJnlLineType{gtables.IntrastatJnlLine_Type.Receipt, gtables.IntrastatJnlLine_Type.Shipment} {
			if strings.EqualFold(typeFilter, t.String()) {
				lineType = int(t)
			}
		}
	}
	if lineType != int(gtables.IntrastatJnlLine_Type.Receipt) && lineType != int(gtables.IntrastatJnlLine_Type.Shipment) {
		return apperrors.FieldErrorText(gtables.IntrastatJnlLineTableName, "type", run.Text("TYPE_ONE"))
	}
	typeText := run.Text(strings.ToUpper(gtables.IntrastatJnlLineType(lineType).String()))
	language := fcodeunits.CurrentLanguage()
	run.SetValue("type_text", typeText)
	run.SetValue("filter_text", rec.TableCaption(language)+": "+rec.FieldCaption("type", language)+": "+typeText)
	return nil
}

// lineOnAfterGetRecord: an empty line is left out; a line declared has its fields
func (r *intrastatForm) lineOnAfterGetRecord(rec *gtables.IntrastatJnlLine, run *Run) error {
	if rec.Tariff_no == "" && rec.Country_code == "" && rec.Transaction_type == "" && rec.Transport_method == "" && rec.Total_weight.IsZero() {
		return Skip()
	}
	err := checkIntrastatLine(rec)
	if err != nil {
		return err
	}
	country := gtables.NewCountryRegion(run)
	err = country.GetOrError(rec.Country_code)
	if err != nil {
		return err
	}
	// NoOfRecords: the groups declared (the group footers printed)
	key := intrastatGroupKey(rec)
	if key != r.groupKey {
		r.noOfRecords++
		r.groupKey = key
	}
	run.SetValue("no_of_entries", types.NewDecimalFromInt(int64(r.noOfRecords)))
	run.SetValue("country_intrastat_code", country.Intrastat_code.String())
	run.SetValue("country_name", country.Name.String())
	return nil
}
