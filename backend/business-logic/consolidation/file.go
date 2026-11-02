package consolidation

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"

	fcodeunits "github.com/hansjlachmann/openerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/openerp/backend/foundation/i18n"
	"github.com/hansjlachmann/openerp/backend/foundation/system"
	"github.com/hansjlachmann/openerp/backend/foundation/types"
)

// The consolidation file (Export Consolidation writes it, Import Consolidation from File and
// Consolidation - Test File read it) is text, one record per line, the fields separated by tabs;
// dates as YYYY-MM-DD, amounts with a point:
//
//	<company> <starting date> <ending date> <per day 0/1> <per department 0/1> <per project 0/1>
//	A <G/L account no.> <consol. debit acc.> <consol. credit acc.> <method 1 current, 2 historical, 3 manual rate>
//	E <G/L account no.> <posting date> <debit amount> <credit amount> <department code> <project code>
//
// An account's A line comes before its E lines.

// FileHeader is the first line of a consolidation file
type FileHeader struct {
	CompanyName              string
	StartDate, EndDate       types.Date
	PerDay, PerDept, PerProj bool
}

// FileLine is an A line (an account: Account set) or an E line (its entries: Entry set)
type FileLine struct {
	Account *SubsidAccount
	Entry   *FileEntry
}

// FileEntry is an E line: the sums of an account's entries for a consolidation line (the
// department and project already their consolidation codes)
type FileEntry struct {
	AccountNo types.Code
	Group
}

// File writes a consolidation file
type File struct {
	buf bytes.Buffer
}

// WriteHeader writes the first line
func (f *File) WriteHeader(h FileHeader) {
	f.writeLine(h.CompanyName, h.StartDate.String(), h.EndDate.String(), flag(h.PerDay), flag(h.PerDept), flag(h.PerProj))
}

// WriteAccount writes an A line
func (f *File) WriteAccount(acc SubsidAccount) {
	f.writeLine("A", acc.No.String(), acc.ConsolDebitAcc.String(), acc.ConsolCreditAcc.String(), strconv.Itoa(int(acc.Method)))
}

// WriteEntry writes an E line
func (f *File) WriteEntry(accountNo types.Code, postingDate types.Date, debit, credit types.Decimal, departmentCode, projectCode types.Code) {
	f.writeLine("E", accountNo.String(), postingDate.String(), debit.String(), credit.String(), departmentCode.String(), projectCode.String())
}

// Bytes is the file's contents
func (f *File) Bytes() []byte {
	return f.buf.Bytes()
}

func (f *File) writeLine(fields ...string) {
	for i, field := range fields {
		if i > 0 {
			f.buf.WriteByte('\t')
		}
		f.buf.WriteString(strings.ReplaceAll(field, "\t", " "))
	}
	f.buf.WriteString("\r\n")
}

func flag(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// unknownFormat is the error of a file that is not a consolidation file
func unknownFormat() error {
	ts := i18n.GetInstance()
	return system.Error(ts.Message("CONSOL_UNKNOWN_FORMAT", fcodeunits.CurrentLanguage()))
}

// ReadFile reads a consolidation file: its header and its lines
func ReadFile(data []byte) (FileHeader, []FileLine, error) {
	var header FileHeader
	var lines []FileLine
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	first := true
	for scanner.Scan() {
		text := strings.TrimRight(scanner.Text(), "\r")
		if first {
			first = false
			fields := strings.Split(text, "\t")
			if len(fields) != 6 {
				return header, nil, unknownFormat()
			}
			header.CompanyName = strings.TrimSpace(fields[0])
			header.StartDate, _ = types.NewDateFromString(fields[1])
			header.EndDate, _ = types.NewDateFromString(fields[2])
			header.PerDay, header.PerDept, header.PerProj = fields[3] == "1", fields[4] == "1", fields[5] == "1"
			if header.CompanyName == "" || header.StartDate.IsZero() || header.EndDate.IsZero() {
				return header, nil, unknownFormat()
			}
			continue
		}
		if text == "" {
			return header, nil, unknownFormat()
		}
		fields := strings.Split(text, "\t")
		switch {
		case fields[0] == "A" && len(fields) == 5:
			method, err := strconv.Atoi(fields[4])
			if err != nil || method < int(CurrentRate) || method > int(ManualRate) {
				return header, nil, unknownFormat()
			}
			lines = append(lines, FileLine{Account: &SubsidAccount{No: types.Code(fields[1]), ConsolDebitAcc: types.Code(fields[2]),
				ConsolCreditAcc: types.Code(fields[3]), Method: Method(method)}})
		case fields[0] == "E" && len(fields) == 7:
			postingDate, err := types.NewDateFromString(fields[2])
			if err != nil || postingDate.IsZero() {
				return header, nil, unknownFormat()
			}
			debit, err := types.NewDecimalFromString(fields[3])
			if err != nil {
				return header, nil, unknownFormat()
			}
			credit, err := types.NewDecimalFromString(fields[4])
			if err != nil {
				return header, nil, unknownFormat()
			}
			// the amounts to consolidate: the additional-currency amounts too, so an account at the
			// historical rate takes them as they are
			lines = append(lines, FileLine{Entry: &FileEntry{AccountNo: types.Code(fields[1]), Group: Group{PostingDate: postingDate,
				DebitAmount: debit, CreditAmount: credit, AcyDebitAmount: debit, AcyCreditAmount: credit,
				DepartmentCode: types.Code(fields[5]), ProjectCode: types.Code(fields[6])}}})
		default:
			return header, nil, unknownFormat()
		}
	}
	err := scanner.Err()
	if err != nil {
		return header, nil, err
	}
	if first {
		return header, nil, unknownFormat()
	}
	return header, lines, nil
}
