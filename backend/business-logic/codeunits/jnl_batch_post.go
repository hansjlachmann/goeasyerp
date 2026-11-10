package codeunits

import (
	"errors"
	"strings"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/bomjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/fajnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/genjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/insurancejnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/itemjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/projectjnl"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/reports"
	"github.com/hansjlachmann/goeasyerp/backend/business-logic/resjnl"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// The journal batch lists' Post and Post and Print (classic codeunits 233/234 Gen. Jnl.-B.Post
// (+Print), 243/244 Item, 253/254 BOM, 273/274 Res., 283/284 Project, 5637/5671 FA. Jnl.,
// 5655/5673 Insurance Jnl.): after asking, every batch in the list (the page's filters: the
// template's batches) is posted in a transaction of its own, as the journal's Post does; Post
// and Print then prints the template's posting report on the register the batch's posting
// wrote. A batch that cannot be posted is left as it is, marked (the list selects it) and named
// in the message. Post needs a template without Force Posting Report, Post and Print one with a
// Posting Report ID.

// jnlBatchKind is one journal's part of the batch codeunits
type jnlBatchKind struct {
	sourceTable string // the batch table the codeunit runs on
	// template checks the template (TESTFIELD) and returns its posting report
	template func(src ftables.Source, templateName types.Code, print bool) (int, error)
	// post posts a batch (the journal's post batch codeunit) and returns its register no.
	post func(src ftables.Source, templateName, batchName types.Code) (int, error)
	// register is the register a posting wrote, filtered to it (SETRECFILTER)
	register func(src ftables.Source, registerNo int) ftables.Table
}

// The codeunits by number
const (
	GenJnlBPostID            = 233
	GenJnlBPostPrintID       = 234
	ItemJnlBPostID           = 243
	ItemJnlBPostPrintID      = 244
	BOMJnlBPostID            = 253
	BOMJnlBPostPrintID       = 254
	ResJnlBPostID            = 273
	ResJnlBPostPrintID       = 274
	ProjectJnlBPostID        = 283
	ProjectJnlBPostPrintID   = 284
	FAJnlBPostID             = 5637
	FAJnlBPostPrintID        = 5671
	InsuranceJnlBPostID      = 5655
	InsuranceJnlBPostPrintID = 5673
)

func init() {
	register := func(postID, postPrintID int, name string, kind jnlBatchKind) {
		Register(postID, name+"-jnl-b-post", func(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
			return &JnlBatchPost{id: postID, name: name + "-jnl-b-post", kind: kind, db: db, company: company, dbType: dbType}
		})
		Register(postPrintID, name+"-jnl-b-post-print", func(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
			return &JnlBatchPost{id: postPrintID, name: name + "-jnl-b-post-print", kind: kind, print: true, db: db, company: company, dbType: dbType}
		})
	}
	register(GenJnlBPostID, GenJnlBPostPrintID, "gen", genJnlBatchKind)
	register(ItemJnlBPostID, ItemJnlBPostPrintID, "item", itemJnlBatchKind)
	register(BOMJnlBPostID, BOMJnlBPostPrintID, "bom", bomJnlBatchKind)
	register(ResJnlBPostID, ResJnlBPostPrintID, "res", resJnlBatchKind)
	register(ProjectJnlBPostID, ProjectJnlBPostPrintID, "project", projectJnlBatchKind)
	register(FAJnlBPostID, FAJnlBPostPrintID, "fa", faJnlBatchKind)
	register(InsuranceJnlBPostID, InsuranceJnlBPostPrintID, "insurance", insuranceJnlBatchKind)
}

// JnlBatchPost is one of the batch codeunits
type JnlBatchPost struct {
	id      int
	name    string
	kind    jnlBatchKind
	print   bool
	db      database.Executor
	company string
	dbType  database.DBType
}

func (c *JnlBatchPost) ID() int                    { return c.id }
func (c *JnlBatchPost) Name() string               { return c.name }
func (c *JnlBatchPost) SourceTable() string        { return c.kind.sourceTable }
func (c *JnlBatchPost) UsesProgress() bool         { return true } // it asks (Confirm)
func (c *JnlBatchPost) GetDB() database.Executor   { return c.db }
func (c *JnlBatchPost) GetCompany() string         { return c.company }
func (c *JnlBatchPost) GetDBType() database.DBType { return c.dbType }

// jnlBatch is a batch record the codeunit runs on (with the page's filters)
type jnlBatch interface {
	ftables.Table
	FindSet() bool
	Next(steps ...int) bool
}

// Run posts the batches in the list (the record with the page's filters)
func (c *JnlBatchPost) Run(record interface{}) (fcodeunits.Result, error) {
	jnlBatchRec, ok := record.(jnlBatch)
	if !ok {
		return fcodeunits.Result{}, errors.New("a journal batch codeunit runs on journal batches")
	}
	ts := i18n.GetInstance()
	lang := fcodeunits.CurrentLanguage()

	type batchKey struct {
		templateName, name types.Code
		key                string // the record's key in the list
	}
	var batches []batchKey
	for found := jnlBatchRec.FindSet(); found; found = jnlBatchRec.Next() {
		data := jnlBatchRec.ToMap()
		batches = append(batches, batchKey{types.Code(stringOf(data["journal_template_name"])), types.Code(stringOf(data["name"])), ftables.KeyString(jnlBatchRec)})
	}
	if len(batches) == 0 {
		return system.Message(ts.Message("GEN_JNL_NOTHING_TO_POST", lang)), nil
	}
	src := ftables.DBSource(c.db, c.company, c.dbType)
	postingReportID, err := c.kind.template(src, batches[0].templateName, c.print)
	if err != nil {
		return fcodeunits.Result{}, err
	}
	question := "JNL_BATCHES_POST_CONFIRM"
	if c.print {
		question = "JNL_BATCHES_POST_PRINT_CONFIRM"
	}
	if !system.Confirm(ts.Message(question, lang)) {
		return fcodeunits.Result{Success: true}, nil
	}

	var jnlWithErrors []string
	var marked []string
	var printouts []reports.Result
	for _, batch := range batches {
		registerNo, ok := c.postBatch(batch.templateName, batch.name)
		if !ok {
			jnlWithErrors = append(jnlWithErrors, batch.name.String())
			marked = append(marked, batch.key)
			continue
		}
		if c.print && registerNo != 0 {
			register := c.kind.register(src, registerNo)
			if register != nil {
				printed, err := printPostingReport(src, postingReportID, register)
				if err != nil {
					return fcodeunits.Result{}, err
				}
				printouts = append(printouts, printed...)
			}
		}
	}

	var result fcodeunits.Result
	if len(jnlWithErrors) == 0 {
		result = system.Message(ts.Message("JNL_BATCHES_POSTED", lang))
	} else {
		result = system.Message(ts.Message("JNL_BATCHES_NOT_ALL_POSTED", lang), strings.Join(jnlWithErrors, ", "))
	}
	result.Refresh = true
	result.Mark = marked
	return reports.WithPrintouts(result, printouts), nil
}

// postBatch posts one batch in its own transaction (classic IF JnlPostBatch.RUN(JnlLine));
// false when it could not be posted
func (c *JnlBatchPost) postBatch(templateName, batchName types.Code) (int, bool) {
	tx, err := ftables.BeginTransaction(c.db)
	if err != nil {
		return 0, false
	}
	registerNo, err := c.kind.post(ftables.DBSource(tx, c.company, c.dbType), templateName, batchName)
	if err != nil {
		_ = ftables.Rollback(tx)
		return 0, false
	}
	return registerNo, ftables.Commit(tx) == nil
}

// stringOf is a map value as text
func stringOf(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case types.Code:
		return s.String()
	case nil:
		return ""
	}
	return ""
}

// templateCheck is what the template TESTFIELDs: Post needs Force Posting Report off, Post and
// Print a Posting Report ID
type templateCheck interface {
	TestField(fieldName string, value ...interface{}) error
}

func checkJnlTemplate(template templateCheck, print bool) error {
	if print {
		return template.TestField("posting_report_id")
	}
	return template.TestField("force_posting_report", false)
}

var genJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.GenJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		genJnlTemplate := gtables.NewGenJournalTemplate(src)
		err := genJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(genJnlTemplate, print)
		}
		return genJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		genJnlLine := gtables.NewGenJournalLine(src)
		genJnlLine.Journal_template_name, genJnlLine.Journal_batch_name, genJnlLine.Line_no = templateName, batchName, 1
		posted, err := genjnl.NewGenJnlPostBatch(src).Run(genJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		glReg := gtables.NewGLRegister(src)
		if !glReg.Get(registerNo) {
			return nil
		}
		glReg.SetRecFilter()
		return glReg
	},
}

var itemJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.ItemJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		itemJnlTemplate := gtables.NewItemJournalTemplate(src)
		err := itemJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(itemJnlTemplate, print)
		}
		return itemJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		itemJnlLine := gtables.NewItemJournalLine(src)
		itemJnlLine.Journal_template_name, itemJnlLine.Journal_batch_name, itemJnlLine.Line_no = templateName, batchName, 1
		posted, err := itemjnl.NewItemJnlPostBatch(src).Run(itemJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		itemReg := gtables.NewItemRegister(src)
		if !itemReg.Get(registerNo) {
			return nil
		}
		itemReg.SetRecFilter()
		return itemReg
	},
}

var bomJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.BOMJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		bomJnlTemplate := gtables.NewBOMJournalTemplate(src)
		err := bomJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(bomJnlTemplate, print)
		}
		return bomJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		bomJnlLine := gtables.NewBOMJournalLine(src)
		bomJnlLine.Journal_template_name, bomJnlLine.Journal_batch_name, bomJnlLine.Line_no = templateName, batchName, 1
		posted, err := bomjnl.NewBOMJnlPostBatch(src).Run(bomJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		bomReg := gtables.NewBOMRegister(src)
		if !bomReg.Get(registerNo) {
			return nil
		}
		bomReg.SetRecFilter()
		return bomReg
	},
}

var resJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.ResJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		resJnlTemplate := gtables.NewResJournalTemplate(src)
		err := resJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(resJnlTemplate, print)
		}
		return resJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		resJnlLine := gtables.NewResJournalLine(src)
		resJnlLine.Journal_template_name, resJnlLine.Journal_batch_name, resJnlLine.Line_no = templateName, batchName, 1
		posted, err := resjnl.NewResJnlPostBatch(src).Run(resJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		resReg := gtables.NewResourceRegister(src)
		if !resReg.Get(registerNo) {
			return nil
		}
		resReg.SetRecFilter()
		return resReg
	},
}

var projectJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.ProjectJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		projectJnlTemplate := gtables.NewProjectJournalTemplate(src)
		err := projectJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(projectJnlTemplate, print)
		}
		return projectJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		projectJnlLine := gtables.NewProjectJournalLine(src)
		projectJnlLine.Journal_template_name, projectJnlLine.Journal_batch_name, projectJnlLine.Line_no = templateName, batchName, 1
		posted, err := projectjnl.NewProjectJnlPostBatch(src).Run(projectJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		projectReg := gtables.NewProjectRegister(src)
		if !projectReg.Get(registerNo) {
			return nil
		}
		projectReg.SetRecFilter()
		return projectReg
	},
}

var faJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.FAJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		faJnlTemplate := gtables.NewFAJournalTemplate(src)
		err := faJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(faJnlTemplate, print)
		}
		return faJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		faJnlLine := gtables.NewFAJournalLine(src)
		faJnlLine.Journal_template_name, faJnlLine.Journal_batch_name, faJnlLine.Line_no = templateName, batchName, 1
		posted, err := fajnl.NewFAJnlPostBatch(src).Run(faJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		faReg := gtables.NewFARegister(src)
		if !faReg.Get(registerNo) {
			return nil
		}
		faReg.SetRecFilter()
		return faReg
	},
}

var insuranceJnlBatchKind = jnlBatchKind{
	sourceTable: gtables.InsuranceJournalBatchTableName,
	template: func(src ftables.Source, templateName types.Code, print bool) (int, error) {
		insuranceJnlTemplate := gtables.NewInsuranceJournalTemplate(src)
		err := insuranceJnlTemplate.GetOrError(templateName)
		if err == nil {
			err = checkJnlTemplate(insuranceJnlTemplate, print)
		}
		return insuranceJnlTemplate.Posting_report_id, err
	},
	post: func(src ftables.Source, templateName, batchName types.Code) (int, error) {
		insuranceJnlLine := gtables.NewInsuranceJournalLine(src)
		insuranceJnlLine.Journal_template_name, insuranceJnlLine.Journal_batch_name, insuranceJnlLine.Line_no = templateName, batchName, 1
		posted, err := insurancejnl.PostBatch(src, insuranceJnlLine)
		return posted.RegisterNo, err
	},
	register: func(src ftables.Source, registerNo int) ftables.Table {
		insuranceReg := gtables.NewInsuranceRegister(src)
		if !insuranceReg.Get(registerNo) {
			return nil
		}
		insuranceReg.SetRecFilter()
		return insuranceReg
	},
}
