// Package noseries assigns numbers from number series (No. Series, No. Series Line, No. Series
// Relationship): the classic codeunit NoSeriesManagement. A document or card takes its next
// number with InitSeries (OnInsert), the user picks another related series with SelectSeries
// (AssistEdit), and a number typed by hand is allowed with TestManual.
package noseries

import (
	"math/big"
	"strings"

	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	apperrors "github.com/hansjlachmann/goeasyerp/backend/foundation/errors"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/system"
	ftables "github.com/hansjlachmann/goeasyerp/backend/foundation/tables"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
)

// NoSeriesPageID is the page the user picks a series on (No. Series, lookup mode)
const NoSeriesPageID = 456

// maxNoLength is the length of a number (Code20)
const maxNoLength = 20

// Manager is a NoSeriesManagement variable: it keeps the series it last worked on (SetSeries
// continues with it) and, for GetNextNo without ModifySeries, the line it took the number
// from (SaveNoSeries stores it). One Manager per document, like the classic variable.
type Manager struct {
	db      database.Executor
	company string
	dbType  database.DBType

	noSeries     gtables.NoSeries
	noSeriesCode types.Code // the default series the related ones are filtered for

	lastLine    gtables.NoSeriesLine
	hasLastLine bool

	warningSeriesCode types.Code // the series warned about (once per Manager)
	trying            bool       // TryGetNextNo: no warning

	// Warning (optional) is told when a series reaches its Warning No. (classic Message);
	// the number is assigned all the same. Without it the user is told (fcodeunits.Notify)
	Warning func(err error)
}

// New returns a Manager working in the database and company of a record (classic
// NoSeriesMgt : Codeunit 396): noSeriesMgt := noseries.New(rec)
func New(from ftables.Source) *Manager {
	m := &Manager{db: from.GetDB(), company: from.GetCompany(), dbType: from.GetDBType()}
	m.noSeries.InitWithDBType(m.db, m.company, m.dbType)
	m.lastLine.InitWithDBType(m.db, m.company, m.dbType)
	return m
}

// GetDB, GetCompany and GetDBType: records are opened in the Manager's database and company
// (gtables.NewNoSeriesLine(m))
func (m *Manager) GetDB() database.Executor   { return m.db }
func (m *Manager) GetCompany() string         { return m.company }
func (m *Manager) GetDBType() database.DBType { return m.dbType }

// getNoSeries reads a series into m.noSeries (Get without IF: an error when it does not exist)
func (m *Manager) getNoSeries(code types.Code) error {
	if !m.noSeries.Get(code) {
		return apperrors.TableRecordNotFound(gtables.NoSeriesTableName, code.String())
	}
	return nil
}

// TestManual returns an error unless numbers of the default series may be entered by hand
// (Manual Nos.); no default series allows any number
func (m *Manager) TestManual(defaultNoSeriesCode types.Code) error {
	if defaultNoSeriesCode != "" {
		err := m.getNoSeries(defaultNoSeriesCode)
		if err != nil {
			return err
		}
		if !m.noSeries.Manual_nos {
			return apperrors.NoSeriesManual(gtables.NoSeriesTableName, "manual_nos", m.noSeries.Code.String())
		}
	}
	return nil
}

// InitSeries gives a new record its number: when newNo is blank, the next number of the old
// series (the record's No. Series, when it is the default or related to it) or of the default
// series, which must assign numbers automatically (Default Nos.) — newSeriesCode is set to the
// series used. A number typed by hand (newNo filled in) is checked with TestManual.
func (m *Manager) InitSeries(defaultNoSeriesCode, oldNoSeriesCode types.Code, newDate types.Date, newNo, newNoSeriesCode *types.Code) error {
	if *newNo != "" {
		return m.TestManual(defaultNoSeriesCode)
	}
	err := m.getNoSeries(defaultNoSeriesCode)
	if err != nil {
		return err
	}
	if !m.noSeries.Default_nos {
		return apperrors.NoSeriesDefault(gtables.NoSeriesTableName, "default_nos", m.noSeries.Code.String())
	}
	if oldNoSeriesCode != "" {
		m.noSeriesCode = defaultNoSeriesCode
		err = m.filterSeries()
		if err != nil {
			return err
		}
		m.noSeries.Code = oldNoSeriesCode
		if !m.noSeries.Find() {
			err = m.getNoSeries(defaultNoSeriesCode)
			if err != nil {
				return err
			}
		}
	}
	no, err := m.GetNextNo(m.noSeries.Code, newDate, true)
	if err != nil {
		return err
	}
	*newNo = no
	*newNoSeriesCode = m.noSeries.Code
	return nil
}

// SetDefaultSeries sets newNoSeriesCode to a series when it assigns numbers automatically
// (Default Nos.)
func (m *Manager) SetDefaultSeries(newNoSeriesCode *types.Code, noSeriesCode types.Code) error {
	if noSeriesCode != "" {
		err := m.getNoSeries(noSeriesCode)
		if err != nil {
			return err
		}
		if m.noSeries.Default_nos {
			*newNoSeriesCode = m.noSeries.Code
		}
	}
	return nil
}

// SelectSeries lets the user pick a series — the default or one related to it — on the No.
// Series page (lookup mode), positioned on newNoSeriesCode or else oldNoSeriesCode; true and
// newNoSeriesCode set when the user picked one
func (m *Manager) SelectSeries(defaultNoSeriesCode, oldNoSeriesCode types.Code, newNoSeriesCode *types.Code) (bool, error) {
	m.noSeriesCode = defaultNoSeriesCode
	err := m.filterSeries()
	if err != nil {
		return false, err
	}
	if *newNoSeriesCode == "" {
		if oldNoSeriesCode != "" {
			m.noSeries.Code = oldNoSeriesCode
		}
	} else {
		m.noSeries.Code = *newNoSeriesCode
	}
	if system.RunModal(NoSeriesPageID, &m.noSeries, true) == system.ActionLookupOK {
		*newNoSeriesCode = m.noSeries.Code
		return true, nil
	}
	return false, nil
}

// LookupSeries is SelectSeries positioned on the current series (newNoSeriesCode)
func (m *Manager) LookupSeries(defaultNoSeriesCode types.Code, newNoSeriesCode *types.Code) (bool, error) {
	return m.SelectSeries(defaultNoSeriesCode, *newNoSeriesCode, newNoSeriesCode)
}

// TestSeries returns an error unless newNoSeriesCode is the default series or related to it
func (m *Manager) TestSeries(defaultNoSeriesCode, newNoSeriesCode types.Code) error {
	m.noSeriesCode = defaultNoSeriesCode
	err := m.filterSeries()
	if err != nil {
		return err
	}
	m.noSeries.Code = newNoSeriesCode
	if !m.noSeries.Find() {
		return apperrors.TableRecordNotFound(gtables.NoSeriesTableName, newNoSeriesCode.String())
	}
	return nil
}

// SetSeries sets newNo to the next number of the series picked last (SelectSeries), which must
// still be the default series or related to it
func (m *Manager) SetSeries(newNo *types.Code) error {
	code := m.noSeries.Code
	err := m.filterSeries()
	if err != nil {
		return err
	}
	m.noSeries.Code = code
	if !m.noSeries.Find() {
		return apperrors.TableRecordNotFound(gtables.NoSeriesTableName, code.String())
	}
	no, err := m.GetNextNo(m.noSeries.Code, types.Date{}, true)
	if err != nil {
		return err
	}
	*newNo = no
	return nil
}

// filterSeries filters m.noSeries to the default series m.noSeriesCode and the series related
// to it (No. Series Relationship)
func (m *Manager) filterSeries() error {
	m.noSeries.Reset()
	codes := []string{}
	rel := gtables.NewNoSeriesRelationship(m)
	rel.SetRange("code", m.noSeriesCode)
	for found := rel.FindSet(); found; found = rel.Next() {
		codes = append(codes, rel.Series_code.String())
	}
	err := m.getNoSeries(m.noSeriesCode)
	if err != nil {
		return err
	}
	codes = append(codes, m.noSeriesCode.String())
	m.noSeries.SetFilter("code", strings.Join(codes, "|"))
	return nil
}

// IssuedBy returns the series — the default one or one related to it — whose line issued no
// last (its Last No. Used): a record saved with the number SelectSeries/SetSeries gave it
// (AssistEdit) belongs to that series; false when none did
func (m *Manager) IssuedBy(defaultNoSeriesCode types.Code, no types.Code) (types.Code, bool) {
	if defaultNoSeriesCode == "" || no == "" {
		return "", false
	}
	m.noSeriesCode = defaultNoSeriesCode
	err := m.filterSeries()
	if err != nil {
		return "", false
	}
	line := gtables.NewNoSeriesLine(m)
	line.SetFilter("series_code", m.noSeries.GetFilter("code"))
	line.SetRange("last_no_used", no)
	if !line.FindFirst() {
		return "", false
	}
	return line.Series_code, true
}

// GetNextNo returns the next number of a series on a date (blank: the work date), from the
// line open on that date. With modifySeries the line is locked and stored; without, the
// Manager keeps the line and continues with it at the next call (SaveNoSeries stores it).
func (m *Manager) GetNextNo(noSeriesCode types.Code, seriesDate types.Date, modifySeries bool) (types.Code, error) {
	if seriesDate.IsZero() {
		seriesDate = system.WorkDate()
	}
	line := gtables.NewNoSeriesLine(m)
	if modifySeries || !m.hasLastLine {
		if modifySeries {
			line.LockTable()
		}
		err := m.getNoSeries(noSeriesCode)
		if err != nil {
			return "", err
		}
		SetNoSeriesLineFilter(line, noSeriesCode, seriesDate)
		if !line.FindFirst() {
			line.SetRange("starting_date")
			if line.FindFirst() {
				return "", apperrors.NoSeriesNoLineOnDate(noSeriesCode.String(), seriesDate.String())
			}
			return "", apperrors.NoSeriesNoLine(noSeriesCode.String())
		}
	} else {
		line.Copy(&m.lastLine)
	}

	if m.noSeries.Date_order && seriesDate.Before(line.Last_date_used) {
		return "", apperrors.NoSeriesDateOrder(m.noSeries.Code.String(), line.Last_date_used.String())
	}
	line.Last_date_used = seriesDate
	if line.Last_no_used == "" {
		err := line.TestField("starting_no")
		if err != nil {
			return "", err
		}
		line.Last_no_used = line.Starting_no
	} else if line.Increment_by_no <= 1 {
		line.Last_no_used = types.Code(system.IncStr(line.Last_no_used.String()))
	} else {
		no, err := IncrementNoText(line.Last_no_used, line.Increment_by_no)
		if err != nil {
			return "", err
		}
		line.Last_no_used = no
	}
	if line.Ending_no != "" && line.Last_no_used > line.Ending_no {
		return "", apperrors.NoSeriesEndingNo(line.Ending_no.String(), noSeriesCode.String())
	}
	if line.Ending_no != "" && line.Warning_no != "" && line.Last_no_used >= line.Warning_no &&
		noSeriesCode != m.warningSeriesCode && !m.trying {
		m.warningSeriesCode = noSeriesCode
		warning := apperrors.NoSeriesEndingNo(line.Ending_no.String(), noSeriesCode.String())
		if m.Warning != nil {
			m.Warning(warning)
		} else {
			fcodeunits.Notify(warning.Message(fcodeunits.CurrentLanguage()))
		}
	}
	err := line.Validate("open", line.Open)
	if err != nil {
		return "", err
	}

	if modifySeries {
		err := line.ModifyOrError(false)
		if err != nil {
			return "", err
		}
	} else {
		m.lastLine.Copy(line)
		m.hasLastLine = true
	}
	return line.Last_no_used, nil
}

// TryGetNextNo returns the next number of a series on a date without using it (the line is
// not changed), "" when the series cannot give one
func (m *Manager) TryGetNextNo(noSeriesCode types.Code, seriesDate types.Date) types.Code {
	try := New(m)
	try.trying = true
	no, err := try.GetNextNo(noSeriesCode, seriesDate, false)
	if err != nil {
		return ""
	}
	return no
}

// SaveNoSeries stores the line GetNextNo without ModifySeries took numbers from
func (m *Manager) SaveNoSeries() error {
	if m.hasLastLine && m.lastLine.Series_code != "" {
		err := m.lastLine.ModifyOrError(false)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetNoSeriesLineFilter filters line to the open lines of a series with the latest starting
// date on or before startDate (blank: the work date)
func SetNoSeriesLineFilter(line *gtables.NoSeriesLine, noSeriesCode types.Code, startDate types.Date) {
	if startDate.IsZero() {
		startDate = system.WorkDate()
	}
	line.Reset()
	line.SetCurrentKey("series_code", "starting_date")
	line.SetRange("series_code", noSeriesCode)
	line.SetRange("starting_date", types.Date{}, startDate)
	if line.FindLast() {
		line.SetRange("starting_date", line.Starting_date)
		line.SetRange("open", true)
	}
}

// IncrementNoText adds incrementBy to the number part of no, keeping its length
// ("CUST00010" + 10 → "CUST00020")
func IncrementNoText(no types.Code, incrementBy int) (types.Code, error) {
	s := no.String()
	start, end := getIntegerPos(s)
	n, ok := new(big.Int).SetString(s[start-1:end], 10)
	if !ok {
		return no, apperrors.NoSeriesTooLong(s)
	}
	n.Add(n, big.NewInt(int64(incrementBy)))
	out, err := replaceNoText(s, n.String(), 0, start, end)
	return types.Code(out), err
}

// UpdateNoSeriesLine gives the number fields of a line (Starting No., Ending No., Last No.
// Used, Warning No.) the format of newNo, entered in field (its OnValidate): the same text
// around the number and the number part's length. newNo must contain a number, and Last No.
// Used must keep its number (else the formats differ).
func UpdateNoSeriesLine(line *gtables.NoSeriesLine, newNo types.Code, field string) error {
	if newNo != "" {
		n := newNo.String()
		if system.IncStr(n) == "" {
			return apperrors.NoSeriesNoNumber(gtables.NoSeriesLineTableName, field)
		}
		noSeriesLine2 := *line // the line before (its Last No. Used)
		length := 0
		if n != getNoText(n) {
			length = len(getNoText(n))
			for _, no := range []types.Code{line.Starting_no, line.Ending_no, line.Last_no_used, line.Warning_no} {
				updateLength(no.String(), &length)
			}
		}
		for _, no := range []*types.Code{&line.Starting_no, &line.Ending_no, &line.Last_no_used, &line.Warning_no} {
			err := updateNo(no, n, length)
			if err != nil {
				return err
			}
		}
		if field != "last_no_used" && line.Last_no_used != noSeriesLine2.Last_no_used {
			return apperrors.NoSeriesNumberFormat(gtables.NoSeriesLineTableName, field, "last_no_used")
		}
	}
	return nil
}

// updateLength raises maxLength to the length of no's number part without leading zeros
func updateLength(no string, maxLength *int) {
	if no == "" {
		return
	}
	l := len(strings.TrimLeft(getNoText(no), "0"))
	if l > *maxLength {
		*maxLength = l
	}
}

// updateNo puts no's number (without leading zeros) into newNo's format, its number part
// length digits long; length 0 leaves only the number
func updateNo(no *types.Code, newNo string, length int) error {
	if *no != "" {
		number := strings.TrimLeft(getNoText(no.String()), "0")
		*no = types.Code(number)
		if length != 0 {
			start, end := getIntegerPos(newNo)
			out, err := replaceNoText(newNo, number, length, start, end)
			if err != nil {
				return err
			}
			*no = types.Code(out)
		}
	}
	return nil
}

// replaceNoText replaces the number part of no (positions start..end, 1-based) with newNo,
// padded with zeros to the old part's length (at least fixedLength)
func replaceNoText(no, newNo string, fixedLength, start, end int) (string, error) {
	startNo, endNo := "", ""
	if start > 1 {
		startNo = no[:start-1]
	}
	if end < len(no) {
		endNo = no[end:]
	}
	oldLength := end - start + 1
	if fixedLength > oldLength {
		oldLength = fixedLength
	}
	zeros := ""
	if oldLength > len(newNo) {
		zeros = strings.Repeat("0", oldLength-len(newNo))
	}
	if len(startNo)+len(zeros)+len(newNo)+len(endNo) > maxNoLength {
		return no, apperrors.NoSeriesTooLong(no)
	}
	return startNo + zeros + newNo + endNo, nil
}

// getNoText is the last number in no ("" when it has none)
func getNoText(no string) string {
	start, end := getIntegerPos(no)
	if start == 0 {
		return ""
	}
	return no[start-1 : end]
}

// getIntegerPos finds the last number in no: its first and last position (1-based), 0 and 0
// when no has no digits
func getIntegerPos(no string) (start, end int) {
	for i := len(no); i > 0; i-- {
		isDigit := no[i-1] >= '0' && no[i-1] <= '9'
		if isDigit {
			if end == 0 {
				end = i
			}
			start = i
		} else if start != 0 {
			break
		}
	}
	return start, end
}
