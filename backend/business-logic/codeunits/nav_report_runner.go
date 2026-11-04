package codeunits

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	fcodeunits "github.com/hansjlachmann/goeasyerp/backend/foundation/codeunits"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/database"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/i18n"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/types"
	gtables "github.com/hansjlachmann/goeasyerp/backend/generated/tables"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// NavReportRunner - Codeunit 903: NAV Report Runner
// Executes reports via external NAV proxy service with progress tracking
const NavReportRunnerID = 903
const NavReportRunnerName = "nav-report-runner"

func init() {
	Register(NavReportRunnerID, NavReportRunnerName, NewNavReportRunner)
}

type NavReportRunner struct {
	db      database.Executor
	company string
	dbType  database.DBType
	client  *http.Client
}

// NewNavReportRunner creates a new instance of the codeunit
func NewNavReportRunner(db database.Executor, company string, dbType database.DBType) fcodeunits.Codeunit {
	return &NavReportRunner{
		db:      db,
		company: company,
		dbType:  dbType,
		client: &http.Client{
			Timeout: 60 * time.Minute, // Match NAV proxy WCF timeout for heavy reports
		},
	}
}

// ID returns the codeunit ID
func (c *NavReportRunner) ID() int {
	return NavReportRunnerID
}

// Name returns the codeunit name
func (c *NavReportRunner) Name() string {
	return NavReportRunnerName
}

// SourceTable returns the table this codeunit operates on
func (c *NavReportRunner) SourceTable() string {
	return "Job_Queue"
}

// UsesProgress returns true - this codeunit uses progress dialog updates
func (c *NavReportRunner) UsesProgress() bool {
	return true
}

// StartJobRequest is the request body for starting a NAV job
type StartJobRequest struct {
	JobID       string `json:"JobId"`
	CompanyName string `json:"CompanyName"`
	InputJSON   string `json:"InputJson"`
}

// StartJobResponse is the response from the start job endpoint
type StartJobResponse struct {
	Result string `json:"result"`
}

// CheckJobRequest is the request body for checking a NAV job
type CheckJobRequest struct {
	JobID       string `json:"JobId"`
	CompanyName string `json:"CompanyName"`
}

// CheckJobResponse is the response from the check job endpoint
type CheckJobResponse struct {
	Result   string `json:"result"`
	Progress int    `json:"progress,omitempty"`
}

// GetJobPdfRequest is the request body for fetching a job's PDF
type GetJobPdfRequest struct {
	JobID       string `json:"JobId"`
	CompanyName string `json:"CompanyName"`
	PdfPath     string `json:"PdfPath"`
}

// GetJobPdfResponse is the response from the PDF endpoint
type GetJobPdfResponse struct {
	JobID    string `json:"JobId"`
	FileName string `json:"FileName"`
	Base64   string `json:"Base64"`
}

// Run executes the codeunit - calls external NAV service and tracks progress
func (c *NavReportRunner) Run(record interface{}) (fcodeunits.Result, error) {
	jobQueue := record.(*gtables.JobQueue)

	// Get the dialog for progress updates
	dialog := fcodeunits.GetCurrentDialog()

	// NAV proxy service URL, set per installation (prod.env); default for a proxy on the same host
	baseURL := os.Getenv("NAV_PROXY_URL")
	if baseURL == "" {
		baseURL = "http://localhost:5009"
	}

	// Generate a unique job ID (exactly 20 alphanumeric characters)
	// Format: YYMMDDHHMMSS (12 chars) + random alphanumeric (8 chars)
	jobID := generateJobID()
	log.Printf("[NavReportRunner] Generated job ID: %s", jobID)

	// Read report ID and format from the Parameter field on the Job Queue record
	paramStr := strings.TrimSpace(jobQueue.Parameter.String())
	if paramStr == "" {
		return fcodeunits.Error("Parameter is empty - specify the report ID in the Parameter field"), nil
	}

	jp, err := parseJSONParameter(paramStr)
	if err != nil {
		return fcodeunits.Error("Invalid Parameter field: " + err.Error()), nil
	}

	reportID := jp.ReportID
	if reportID == 0 {
		return fcodeunits.Error("Missing report ID in Parameter field: " + paramStr), nil
	}
	outputFormat := jp.Format

	lang := fcodeunits.CurrentLanguage()
	ts := i18n.GetInstance()

	// Show dialog based on fields configuration
	var filterMap map[string]interface{}
	if len(jp.Fields) > 0 {
		// JSON parameter with fields defined → dynamic input dialog
		resolvedFields := resolveFields(jp.Fields, lang)
		inputResult := fcodeunits.RequestInput(ts.Message("DLG_FILTER_TITLE", lang), resolvedFields)
		if inputResult == nil {
			return fcodeunits.Message(ts.Message("CU_REPORT_CANCELLED", lang)), nil
		}
		filterMap = make(map[string]interface{})
		for k, v := range inputResult {
			filterMap[k] = v
		}
	} else if jp.Fields == nil {
		// Legacy format (no JSON fields key) → hardcoded date dialog
		inputResult := fcodeunits.RequestInput(ts.Message("DLG_FILTER_TITLE", lang), []fcodeunits.InputField{
			{Name: "date", Label: ts.Message("CU_REPORT_DATE", lang), Type: "date", Required: true,
				Default: time.Now().Format("2006-01-02")},
		})
		if inputResult == nil || inputResult["date"] == "" {
			return fcodeunits.Message(ts.Message("CU_REPORT_CANCELLED", lang)), nil
		}
		filterMap = make(map[string]interface{})
		for k, v := range inputResult {
			filterMap[k] = v
		}
	} else {
		// fields: [] (empty array) → confirm dialog before running
		jobDesc := jobQueue.Description.String()
		if !fcodeunits.Confirm(fmt.Sprintf("Start job %s?", jobDesc)) {
			return fcodeunits.Message(ts.Message("CU_REPORT_CANCELLED", lang)), nil
		}
		filterMap = make(map[string]interface{})
	}
	payload := map[string]interface{}{
		"reportId": reportID,
		"format":   outputFormat,
		"filter":   filterMap,
	}
	inputJSONBytes, _ := json.Marshal(payload)
	inputJSON := string(inputJSONBytes)
	log.Printf("[NavReportRunner] Input JSON: %s", inputJSON)

	// Capture job start time for Job Queue Entry logging
	jobStartTime := types.Now()

	// Update progress: Starting
	if dialog != nil {
		dialog.UpdateWithMessage(1, 0, "Starting report generation...")
	}

	// Step 1: Start the job via POST (fire-and-forget)
	// The NAV service POST is synchronous and can take minutes, so we don't wait for it
	startReq := StartJobRequest{
		JobID:       jobID,
		CompanyName: c.company,
		InputJSON:   inputJSON,
	}

	reqBody, err := json.Marshal(startReq)
	if err != nil {
		log.Printf("[NavReportRunner] Failed to marshal request: %v", err)
		return fcodeunits.Error("Failed to marshal request: " + err.Error()), nil
	}

	startURL := baseURL + "/api/nav/startjob"
	log.Printf("[NavReportRunner] POST %s (fire-and-forget) with body: %s", startURL, string(reqBody))

	// startJobResult carries both error and response from the StartJob POST
	type startJobResult struct {
		err    error
		result string // The "result" field from the response JSON
	}

	// Channel to communicate POST result - buffered so goroutine doesn't block
	postResultCh := make(chan startJobResult, 1)

	// Fire POST in background goroutine
	go func() {
		resp, err := c.client.Post(
			startURL,
			"application/json",
			bytes.NewReader(reqBody),
		)
		if err != nil {
			log.Printf("[NavReportRunner] Background POST failed: %v", err)
			postResultCh <- startJobResult{err: err}
			return
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[NavReportRunner] Background POST response (status %d): %s", resp.StatusCode, string(respBody))

		// Check for non-success status codes
		if resp.StatusCode >= 400 {
			// Try to extract the "details" field for a clean error message
			var navErr struct {
				Details string `json:"details"`
			}
			if json.Unmarshal(respBody, &navErr) == nil && navErr.Details != "" {
				postResultCh <- startJobResult{err: fmt.Errorf("%s", navErr.Details)}
			} else {
				postResultCh <- startJobResult{err: fmt.Errorf("NAV service returned status %d: %s", resp.StatusCode, string(respBody))}
			}
			return
		}

		// Parse the result field from response
		var startResp StartJobResponse
		err = json.Unmarshal(respBody, &startResp)
		if err != nil {
			log.Printf("[NavReportRunner] Failed to parse StartJob response: %v", err)
		}

		postResultCh <- startJobResult{result: startResp.Result}
	}()

	// Update progress: Job started
	if dialog != nil {
		dialog.UpdateWithMessage(1, 0, "Report job started, waiting for completion...")
	}

	// Step 2: Poll checkjob endpoint until progress reaches 100%
	pollInterval := 1 * time.Second
	maxWaitTime := 60 * time.Minute
	startTime := time.Now()

	log.Printf("[NavReportRunner] Starting progress polling for job %s", jobID)

	// Wait before first poll to give the NAV service time to register the job
	time.Sleep(2 * time.Second)

	var startJobResultStr string // Captures the StartJob response result (contains PDF path)
	var postDone bool            // Whether the POST goroutine has completed
	var postFailed bool          // Whether the POST returned an error (e.g. timeout)
	var postErrorMsg string      // The original POST error message
	var everSeenProgress bool    // Whether we've ever seen progress > 0
	var lastSeenProgress int     // Last non-zero progress value seen
	zeroProgressPolls := 0       // Consecutive polls with 0% after POST failure
	var checkJobError string     // Error message from checkjob response

	for {
		// Check if we've exceeded max wait time
		if time.Since(startTime) > maxWaitTime {
			log.Printf("[NavReportRunner] Job %s timed out", jobID)
			errMsg := "Report generation timed out after 15 minutes"
			err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
			if err != nil {
				log.Printf("[NavReportRunner] Failed to log job queue entry: %v", err)
			}
			err = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
			if err != nil {
				log.Printf("[NavReportRunner] Failed to set job queue status: %v", err)
			}
			return fcodeunits.Error(errMsg), nil
		}

		// Check for POST result (non-blocking)
		if !postDone {
			select {
			case postResult := <-postResultCh:
				postDone = true
				if postResult.err != nil {
					// POST failed — could be a timeout (job still running) or a real error.
					// Continue polling to check: if progress appears, it's a timeout;
					// if it stays at 0%, the job never started.
					postFailed = true
					postErrorMsg = postResult.err.Error()
					log.Printf("[NavReportRunner] POST failed (continuing to poll): %v", postResult.err)
				} else {
					// POST succeeded, capture the result string (contains PDF path)
					startJobResultStr = postResult.result
					log.Printf("[NavReportRunner] POST succeeded, result: %s", startJobResultStr)
				}
			default:
				// No result yet, continue polling
			}
		}

		// Check progress via checkjob endpoint (POST with JobId + CompanyName)
		checkURL := baseURL + "/api/nav/checkjob"
		checkReq := CheckJobRequest{
			JobID:       jobID,
			CompanyName: c.company,
		}
		checkReqBody, err := json.Marshal(checkReq)
		if err != nil {
			log.Printf("[NavReportRunner] Failed to marshal check request: %v", err)
			continue
		}
		log.Printf("[NavReportRunner] Checking progress: POST %s with body: %s", checkURL, string(checkReqBody))

		checkResp, err := c.client.Post(checkURL, "application/json", bytes.NewReader(checkReqBody))
		if err != nil {
			log.Printf("[NavReportRunner] Progress check error: %v", err)
			if dialog != nil {
				dialog.UpdateWithMessage(1, -1, "Connection error, retrying...")
			}
			continue
		}

		checkBody, err := io.ReadAll(checkResp.Body)
		checkResp.Body.Close()

		if err != nil {
			log.Printf("[NavReportRunner] Failed to read progress response body: %v", err)
			continue
		}

		log.Printf("[NavReportRunner] Progress response: %s", string(checkBody))

		// Parse checkjob for error fields (e.g. {"error":"...","details":"..."})
		var checkErr struct {
			Error   string `json:"error"`
			Details string `json:"details"`
		}
		checkJobError = ""
		if json.Unmarshal(checkBody, &checkErr) == nil && checkErr.Error != "" {
			checkJobError = checkErr.Details
			if checkJobError == "" {
				checkJobError = checkErr.Error
			}
		}

		// Parse progress
		var checkResult CheckJobResponse
		err = json.Unmarshal(checkBody, &checkResult)
		if err != nil {
			checkResult = parseProgressResponse(string(checkBody))
		}

		progress := checkResult.Progress
		if progress == 0 {
			progress = extractProgress(checkResult.Result)
		}

		log.Printf("[NavReportRunner] Progress: %d%%", progress)

		if progress > 0 {
			everSeenProgress = true
			lastSeenProgress = progress
			zeroProgressPolls = 0 // Reset counter when we see real progress
		}

		// If POST already failed and progress is 0, the job either never started
		// or has disappeared (e.g. NAV error mid-execution). Stop after a few polls.
		if postFailed && progress == 0 {
			// If checkjob also returned an error, the job is dead — fail immediately
			if checkJobError != "" {
				log.Printf("[NavReportRunner] POST failed and checkjob error: %s", checkJobError)
				errMsg := "NAV service error: " + postErrorMsg
				err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", err)
				}
				err = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", err)
				}
				return fcodeunits.Error(errMsg), nil
			}
			zeroProgressPolls++
			if zeroProgressPolls >= 5 {
				if everSeenProgress {
					log.Printf("[NavReportRunner] POST failed and job disappeared after reaching %d%% — %d consecutive zero polls", lastSeenProgress, zeroProgressPolls)
				} else {
					log.Printf("[NavReportRunner] POST failed and no progress after %d polls — job never started", zeroProgressPolls)
				}
				errMsg := "NAV service error: " + postErrorMsg
				err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", err)
				}
				err = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", err)
				}
				return fcodeunits.Error(errMsg), nil
			}
		}

		if dialog != nil {
			msg := fmt.Sprintf("Generating report.... %d%%", progress)
			dialog.UpdateWithMessage(1, progress, msg)
		}

		// Only complete when progress reaches 100%
		if progress >= 100 {
			// Process-only report (no PDF output)
			if strings.EqualFold(outputFormat, "NONE") {
				log.Printf("[NavReportRunner] Progress is 100%% for job %s (format=NONE, no PDF)", jobID)
				if dialog != nil {
					dialog.UpdateWithMessage(1, 100, "Process completed!")
				}
				err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", jobStartTime)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", err)
				}
				return fcodeunits.Result{
					Success: true,
					Message: "Process completed successfully",
				}, nil
			}

			log.Printf("[NavReportRunner] Progress is 100%%, fetching PDF for job %s", jobID)

			// Try to get the PDF path. Prefer the StartJob response, but fall back
			// to the CheckJob result (which may contain the path at 100%).
			if startJobResultStr == "" && !postFailed {
				// POST hasn't returned yet — poll with animated progress while waiting
				log.Printf("[NavReportRunner] Waiting for StartJob response to get PDF path...")
				dots := 0
				for {
					select {
					case postResult := <-postResultCh:
						if postResult.err != nil {
							log.Printf("[NavReportRunner] POST failed: %v", postResult.err)
						} else {
							startJobResultStr = postResult.result
							log.Printf("[NavReportRunner] StartJob result received: %s", startJobResultStr)
						}
						goto postDoneWaiting
					default:
						// Animate: "Saving PDF.", "Saving PDF..", "Saving PDF..."
						dots = (dots % 3) + 1
						if dialog != nil {
							msg := "Saving PDF" + strings.Repeat(".", dots)
							dialog.UpdateWithMessage(1, 100, msg)
						}
						time.Sleep(500 * time.Millisecond)
					}
				}
			postDoneWaiting:
			}

			// Determine PDF path: from startjob result, or from checkjob result at 100%
			var pdfPath string
			if startJobResultStr != "" {
				pdfPath = extractPdfPath(startJobResultStr)
			} else {
				// StartJob timed out — try the checkjob result string
				log.Printf("[NavReportRunner] StartJob unavailable, trying checkjob result: %s", checkResult.Result)
				pdfPath = extractPdfPath(checkResult.Result)
			}
			log.Printf("[NavReportRunner] Extracted PDF path: %s (from StartJob result: %s)", pdfPath, startJobResultStr)

			if pdfPath == "" {
				log.Printf("[NavReportRunner] No PDF path available — StartJob likely timed out")
				errMsg := "Report completed but PDF path unavailable — the NAV proxy timed out. Try increasing the WCF SendTimeout on the proxy."
				err := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", err)
				}
				err = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if err != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", err)
				}
				return fcodeunits.Error(errMsg), nil
			}

			pdfURL := baseURL + "/api/nav/job/pdf"
			pdfReq := GetJobPdfRequest{
				JobID:       jobID,
				CompanyName: c.company,
				PdfPath:     pdfPath,
			}
			pdfReqBody, err := json.Marshal(pdfReq)
			if err != nil {
				log.Printf("[NavReportRunner] Failed to marshal PDF request: %v", err)
				errMsg := "Failed to marshal PDF request: " + err.Error()
				logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", logErr)
				}
				logErr = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", logErr)
				}
				return fcodeunits.Error(errMsg), nil
			}
			log.Printf("[NavReportRunner] Fetching PDF: POST %s with body: %s", pdfURL, string(pdfReqBody))

			pdfResp, err := c.client.Post(pdfURL, "application/json", bytes.NewReader(pdfReqBody))
			if err != nil {
				log.Printf("[NavReportRunner] PDF fetch error: %v", err)
				errMsg := "Failed to fetch PDF: " + err.Error()
				logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", logErr)
				}
				logErr = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", logErr)
				}
				return fcodeunits.Error(errMsg), nil
			}

			pdfBody, err := io.ReadAll(pdfResp.Body)
			pdfResp.Body.Close()

			if err != nil {
				log.Printf("[NavReportRunner] Failed to read PDF response body: %v", err)
				errMsg := "Failed to read PDF response: " + err.Error()
				logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", logErr)
				}
				logErr = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", logErr)
				}
				return fcodeunits.Error(errMsg), nil
			}

			log.Printf("[NavReportRunner] PDF response (status %d): %d bytes", pdfResp.StatusCode, len(pdfBody))

			if pdfResp.StatusCode != http.StatusOK {
				log.Printf("[NavReportRunner] PDF fetch returned status %d", pdfResp.StatusCode)
				errMsg := fmt.Sprintf("PDF fetch failed with status %d", pdfResp.StatusCode)
				logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", logErr)
				}
				logErr = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", logErr)
				}
				return fcodeunits.Error(errMsg), nil
			}

			var pdfResult GetJobPdfResponse
			err = json.Unmarshal(pdfBody, &pdfResult)
			if err != nil {
				log.Printf("[NavReportRunner] Failed to parse PDF response: %v", err)
				errMsg := "Failed to parse PDF response: " + err.Error()
				logErr := CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Error, errMsg, jobStartTime)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to log job queue entry: %v", logErr)
				}
				logErr = SetJobQueueStatus(c.db, c.company, c.dbType, jobQueue.No, gtables.JobQueue_Status.Error)
				if logErr != nil {
					log.Printf("[NavReportRunner] Failed to set job queue status: %v", logErr)
				}
				return fcodeunits.Error(errMsg), nil
			}

			if dialog != nil {
				dialog.UpdateWithMessage(1, 100, "PDF ready!")
			}

			// Log successful execution to Job Queue Entries
			err = CreateJobQueueEntry(c.db, c.company, c.dbType, jobQueue, gtables.JobQueueEntry_Status.Success, "", jobStartTime)
			if err != nil {
				log.Printf("[NavReportRunner] Failed to log job queue entry: %v", err)
			}

			return fcodeunits.Result{
				Success: true,
				Message: "Report generated successfully",
				Data: map[string]interface{}{
					"pdf":      pdfResult.Base64,
					"filename": pdfResult.FileName,
				},
			}, nil
		}

		// Wait before next poll
		time.Sleep(pollInterval)
	}
}

// parseParameter parses structured parameter string into key-value map.
// Supports: "7350" (legacy plain number), "reportId=7350", "reportId=7350;format=NONE"
func parseParameter(param string) map[string]string {
	result := make(map[string]string)
	param = strings.TrimSpace(param)

	// Legacy: plain number = report ID with PDF
	_, err := strconv.Atoi(param)
	if err == nil {
		result["reportId"] = param
		result["format"] = "PDF"
		return result
	}

	// Structured: key=value;key=value (keys are case-insensitive, normalized to camelCase)
	for _, part := range strings.Split(param, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			// Normalize known keys to canonical form
			switch strings.ToLower(key) {
			case "reportid":
				result["reportId"] = value
			case "format":
				result["format"] = value
			default:
				result[key] = value
			}
		}
	}

	// Default format to PDF if not specified
	if result["format"] == "" {
		result["format"] = "PDF"
	}

	return result
}

// JobParameter represents the parsed JSON parameter for a job queue entry
type JobParameter struct {
	ReportID int                     `json:"reportId"`
	Format   string                  `json:"format"`
	Fields   []fcodeunits.InputField `json:"fields"`
}

// parseJSONParameter parses the parameter string as JSON or falls back to legacy key=value format.
// For JSON: parses into JobParameter directly.
// For legacy: uses parseParameter() and returns JobParameter with Fields=nil (signals hardcoded date dialog).
func parseJSONParameter(param string) (*JobParameter, error) {
	param = strings.TrimSpace(param)

	if strings.HasPrefix(param, "{") {
		var jp JobParameter
		err := json.Unmarshal([]byte(param), &jp)
		if err != nil {
			return nil, fmt.Errorf("invalid JSON parameter: %w", err)
		}
		if jp.Format == "" {
			jp.Format = "PDF"
		}
		return &jp, nil
	}

	// Legacy format → parse as key=value
	legacy := parseParameter(param)
	reportID, _ := strconv.Atoi(legacy["reportId"])
	format := legacy["format"]
	if format == "" {
		format = "PDF"
	}
	return &JobParameter{
		ReportID: reportID,
		Format:   format,
		Fields:   nil, // nil signals legacy mode (hardcoded date dialog)
	}, nil
}

// resolveFields resolves i18n labels and magic defaults for input fields.
func resolveFields(fields []fcodeunits.InputField, lang string) []fcodeunits.InputField {
	ts := i18n.GetInstance()
	resolved := make([]fcodeunits.InputField, len(fields))
	for i, f := range fields {
		resolved[i] = f
		// Try i18n key first; if no translation found, use label as-is
		translated := ts.Message(f.Label, lang)
		if translated != f.Label {
			resolved[i].Label = translated
		}
		// Magic default: "today" → current date ISO
		if f.Type == "date" && strings.EqualFold(f.Default, "today") {
			resolved[i].Default = time.Now().Format("2006-01-02")
		}
	}
	return resolved
}

// parseProgressResponse parses the response body into CheckJobResponse
func parseProgressResponse(body string) CheckJobResponse {
	var result CheckJobResponse

	// Try JSON parse first
	err := json.Unmarshal([]byte(body), &result)
	if err == nil {
		return result
	}

	// Try to extract from {"result": "Progress X"} format
	var simpleResult struct {
		Result string `json:"result"`
	}
	err = json.Unmarshal([]byte(body), &simpleResult)
	if err == nil {
		result.Result = simpleResult.Result
		result.Progress = extractProgress(simpleResult.Result)
	}

	return result
}

// extractProgress extracts the progress number from "Progress X" string
// Handles various formats: "Progress 45", "Progress: 45", "progress 45", etc.
func extractProgress(s string) int {
	// Try multiple patterns
	patterns := []string{
		`[Pp]rogress[:\s]+(\d+)`, // "Progress 45" or "Progress: 45"
		`(\d+)\s*%`,              // "45%" or "45 %"
		`(\d+)`,                  // Just a number
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(s)
		if len(matches) >= 2 {
			p, err := strconv.Atoi(matches[1])
			if err == nil {
				return p
			}
		}
	}
	return 0
}

// extractPdfPath extracts the PDF file path from the StartJob result string.
// Expected format: "Job completed duration:  ms. PDF: C:\...\file.pdf"
func extractPdfPath(result string) string {
	// Look for "PDF: " marker (the standard format from NAV proxy)
	pdfMarkerRe := regexp.MustCompile(`(?i)PDF:\s*(.+\.pdf)`)
	matches := pdfMarkerRe.FindStringSubmatch(result)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find a Windows file path pattern (path may contain spaces)
	winPathRe := regexp.MustCompile(`([A-Za-z]:\\.+\.pdf)`)
	matches = winPathRe.FindStringSubmatch(result)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}

	// Try to find a Unix file path pattern
	unixPathRe := regexp.MustCompile(`(/.+\.pdf)`)
	matches = unixPathRe.FindStringSubmatch(result)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}

	// No PDF path found
	return ""
}

// generateJobID generates a unique 20-character alphanumeric job ID
// Format: YYMMDDHHMMSS (12 chars) + random alphanumeric (8 chars)
func generateJobID() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	now := time.Now()
	// YYMMDDHHMMSS = 12 characters
	timestamp := now.Format("060102150405")

	// Generate 8 random alphanumeric characters using crypto/rand
	randomBytes := make([]byte, 8)
	_, err := cryptorand.Read(randomBytes)
	if err != nil {
		// Fallback to timestamp-based uniqueness if crypto/rand fails
		log.Printf("[NavReportRunner] crypto/rand failed: %v, using timestamp fallback", err)
		return timestamp + "00000000"
	}

	random := make([]byte, 8)
	for i := range random {
		random[i] = charset[int(randomBytes[i])%len(charset)]
	}

	return timestamp + string(random)
}
