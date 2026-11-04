package scheduler

import (
	"log"

	"github.com/hansjlachmann/goeasyerp/backend/business-logic/jobqueue"
	"github.com/hansjlachmann/goeasyerp/backend/foundation/mail"
)

// currentMailer returns the mail sender to use for notifications. A mailer set
// on the struct (test injection) takes precedence; otherwise configuration is
// loaded from the global SMTP_Setup table. When no enabled setup exists, a
// disabled mailer is returned so Send is a safe no-op.
func (s *Scheduler) currentMailer() mail.Sender {
	if s.mailer != nil {
		return s.mailer
	}
	cfg := s.loadSMTPConfig()
	return mail.NewSMTPMailer(cfg)
}

// loadSMTPConfig reads the SMTP_Setup record (jobqueue.LoadSMTPConfig). A password that
// cannot be decrypted is logged; notifications are then not sent.
func (s *Scheduler) loadSMTPConfig() mail.Config {
	cfg, err := jobqueue.LoadSMTPConfig(s.db, s.dbType)
	if err != nil {
		log.Printf("Job Queue scheduler: SMTP Setup: %v — enter the SMTP password again", err)
	}
	return cfg
}
