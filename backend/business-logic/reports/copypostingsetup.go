package reports

import (
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// Reports 87 Copy - General Posting Setup and 85 Copy - Tax Posting Setup (Copy on the setup
// lists 314/472 and cards 395/473): the setup line the page is on gets the accounts — the groups
// the user ticks — of another line of the setup, after a question. Processing-only reports.
const (
	CopyGenPostingSetupID = 87
	CopyTaxPostingSetupID = 85
)

func init() {
	Register(CopyGenPostingSetupID, "copy-general-posting-setup", func() Report { return &copyGenPostingSetup{} })
	Register(CopyTaxPostingSetupID, "copy-tax-posting-setup", func() Report { return &copyTaxPostingSetup{} })
}

type copyGenPostingSetup struct {
	useGenPostingSetup           *gtables.GeneralPostingSetup
	genPostingSetup              *gtables.GeneralPostingSetup
	sales, purch, invt, projects bool
}

func (r *copyGenPostingSetup) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_FROM",
		Options: []Option{
			{Name: "gen_bus_posting_group", Type: "text", Relation: &OptionRelation{Table: gtables.GenBusinessPostingGroupTableName, Columns: []string{"code", "description"}}},
			{Name: "gen_prod_posting_group", Type: "text", Relation: &OptionRelation{Table: gtables.GenProductPostingGroupTableName, Columns: []string{"code", "description"}}},
			{Name: "sales", Type: "bool", Default: "true"},
			{Name: "purch", Type: "bool", Default: "true"},
			{Name: "invt", Type: "bool", Default: "true"},
			{Name: "projects", Type: "bool", Default: "true"},
		},
		Transaction:  true,
		ContextTable: gtables.GeneralPostingSetupTableName,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:             "General_Posting_Setup",
			Table:            gtables.GeneralPostingSetupTableName,
			OnPreDataItem:    Trigger(r.onPreDataItem),
			OnAfterGetRecord: Trigger(r.onAfterGetRecord),
		}},
	}
}

// onPreReport: the line the page is on (classic SetGenPostingSetup) and the line to copy from
func (r *copyGenPostingSetup) onPreReport(run *Run) error {
	r.useGenPostingSetup = gtables.NewGeneralPostingSetup(run)
	context, ok := run.Context().(*gtables.GeneralPostingSetup)
	if ok {
		r.useGenPostingSetup.Copy(context)
	}
	r.genPostingSetup = gtables.NewGeneralPostingSetup(run)
	err := r.genPostingSetup.GetOrError(map[string]interface{}{
		"gen_bus_posting_group":  types.Code(system.UpperCase(run.Option("gen_bus_posting_group"))),
		"gen_prod_posting_group": types.Code(system.UpperCase(run.Option("gen_prod_posting_group"))),
	})
	if err != nil {
		return err
	}
	r.sales = run.OptionBool("sales")
	r.purch = run.OptionBool("purch")
	r.invt = run.OptionBool("invt")
	r.projects = run.OptionBool("projects")
	return nil
}

func (r *copyGenPostingSetup) onPreDataItem(genPostingSetup *gtables.GeneralPostingSetup, run *Run) error {
	genPostingSetup.SetRange("gen_bus_posting_group", r.useGenPostingSetup.Gen_bus_posting_group)
	genPostingSetup.SetRange("gen_prod_posting_group", r.useGenPostingSetup.Gen_prod_posting_group)
	return nil
}

func (r *copyGenPostingSetup) onAfterGetRecord(rec *gtables.GeneralPostingSetup, run *Run) error {
	genPostingSetup := r.genPostingSetup
	if r.sales {
		rec.Sales_account = genPostingSetup.Sales_account
		rec.Sales_credit_memo_account = genPostingSetup.Sales_credit_memo_account
		rec.Sales_line_disc_account = genPostingSetup.Sales_line_disc_account
		rec.Sales_inv_disc_account = genPostingSetup.Sales_inv_disc_account
		rec.Sales_payment_disc_account = genPostingSetup.Sales_payment_disc_account
	}
	if r.purch {
		rec.Purch_account = genPostingSetup.Purch_account
		rec.Purch_credit_memo_account = genPostingSetup.Purch_credit_memo_account
		rec.Purch_line_disc_account = genPostingSetup.Purch_line_disc_account
		rec.Purch_inv_disc_account = genPostingSetup.Purch_inv_disc_account
		rec.Purch_payment_disc_account = genPostingSetup.Purch_payment_disc_account
	}
	if r.invt {
		rec.Cogs_account = genPostingSetup.Cogs_account
		rec.Inventory_adjmt_account = genPostingSetup.Inventory_adjmt_account
	}
	if r.projects {
		rec.Project_sales_adjmt_account = genPostingSetup.Project_sales_adjmt_account
		rec.Project_cost_adjmt_account = genPostingSetup.Project_cost_adjmt_account
	}
	if system.Confirm(run.Text("CONFIRM")) {
		return rec.ModifyOrError(true)
	}
	return nil
}

type copyTaxPostingSetup struct {
	useTaxPostingSetup     *gtables.TaxPostingSetup
	taxPostingSetup        *gtables.TaxPostingSetup
	taxSetup, sales, purch bool
}

func (r *copyTaxPostingSetup) Definition() *Definition {
	return &Definition{
		OptionsCaption: "COPY_FROM",
		Options: []Option{
			{Name: "tax_bus_posting_group", Type: "text", Relation: &OptionRelation{Table: gtables.TaxBusinessPostingGroupTableName, Columns: []string{"code", "description"}}},
			{Name: "tax_prod_posting_group", Type: "text", Relation: &OptionRelation{Table: gtables.TaxProductPostingGroupTableName, Columns: []string{"code", "description"}}},
			{Name: "tax_setup", Type: "bool", Default: "true"},
			{Name: "sales", Type: "bool", Default: "true"},
			{Name: "purch", Type: "bool", Default: "true"},
		},
		Transaction:  true,
		ContextTable: gtables.TaxPostingSetupTableName,
		OnPreReport:  r.onPreReport,
		DataItems: []*DataItem{{
			Name:             "Tax_Posting_Setup",
			Table:            gtables.TaxPostingSetupTableName,
			OnPreDataItem:    Trigger(r.onPreDataItem),
			OnAfterGetRecord: Trigger(r.onAfterGetRecord),
		}},
	}
}

// onPreReport: the line the page is on (classic SetTaxSetup) and the line to copy from
func (r *copyTaxPostingSetup) onPreReport(run *Run) error {
	r.useTaxPostingSetup = gtables.NewTaxPostingSetup(run)
	context, ok := run.Context().(*gtables.TaxPostingSetup)
	if ok {
		r.useTaxPostingSetup.Copy(context)
	}
	r.taxPostingSetup = gtables.NewTaxPostingSetup(run)
	err := r.taxPostingSetup.GetOrError(map[string]interface{}{
		"tax_bus_posting_group":  types.Code(system.UpperCase(run.Option("tax_bus_posting_group"))),
		"tax_prod_posting_group": types.Code(system.UpperCase(run.Option("tax_prod_posting_group"))),
	})
	if err != nil {
		return err
	}
	r.taxSetup = run.OptionBool("tax_setup")
	r.sales = run.OptionBool("sales")
	r.purch = run.OptionBool("purch")
	return nil
}

func (r *copyTaxPostingSetup) onPreDataItem(taxPostingSetup *gtables.TaxPostingSetup, run *Run) error {
	taxPostingSetup.SetRange("tax_bus_posting_group", r.useTaxPostingSetup.Tax_bus_posting_group)
	taxPostingSetup.SetRange("tax_prod_posting_group", r.useTaxPostingSetup.Tax_prod_posting_group)
	return nil
}

func (r *copyTaxPostingSetup) onAfterGetRecord(rec *gtables.TaxPostingSetup, run *Run) error {
	taxPostingSetup := r.taxPostingSetup
	if r.taxSetup {
		rec.Tax_calculation_type = taxPostingSetup.Tax_calculation_type
		rec.Vat_percent = taxPostingSetup.Vat_percent
		rec.Unrealized_tax_type = taxPostingSetup.Unrealized_tax_type
		rec.Adjust_for_payment_discount = taxPostingSetup.Adjust_for_payment_discount
	}
	if r.sales {
		rec.Sales_vat_account = taxPostingSetup.Sales_vat_account
		rec.Sales_vat_unreal_account = taxPostingSetup.Sales_vat_unreal_account
	}
	if r.purch {
		rec.Purchase_vat_account = taxPostingSetup.Purchase_vat_account
		rec.Purch_vat_unreal_account = taxPostingSetup.Purch_vat_unreal_account
		rec.Reverse_chrg_vat_acc = taxPostingSetup.Reverse_chrg_vat_acc
		rec.Reverse_chrg_vat_unreal_acc = taxPostingSetup.Reverse_chrg_vat_unreal_acc
	}
	if system.Confirm(run.Text("CONFIRM")) {
		return rec.ModifyOrError(true)
	}
	return nil
}
