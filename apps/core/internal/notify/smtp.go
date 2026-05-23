package notify

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// smtpConfig holds SMTP connection parameters read from settings.
type smtpConfig struct {
	host     string
	port     string
	user     string
	password string
	from     string
	to       string
}

func (s *Service) smtpConfig() (smtpConfig, bool) {
	var cfg smtpConfig
	s.db.QueryRow("SELECT value FROM settings WHERE key='SMTP_HOST'").Scan(&cfg.host)         //nolint:errcheck
	s.db.QueryRow("SELECT value FROM settings WHERE key='SMTP_PORT'").Scan(&cfg.port)         //nolint:errcheck
	s.db.QueryRow("SELECT value FROM settings WHERE key='SMTP_USER'").Scan(&cfg.user)         //nolint:errcheck
	s.db.QueryRow("SELECT value FROM settings WHERE key='SMTP_PASSWORD'").Scan(&cfg.password) //nolint:errcheck
	s.db.QueryRow("SELECT value FROM settings WHERE key='SMTP_FROM'").Scan(&cfg.from)         //nolint:errcheck
	s.db.QueryRow("SELECT value FROM settings WHERE key='SMTP_TO'").Scan(&cfg.to)             //nolint:errcheck
	if cfg.host == "" || cfg.to == "" {
		return cfg, false
	}
	if cfg.port == "" {
		cfg.port = "587"
	}
	if cfg.from == "" {
		cfg.from = cfg.user
	}
	return cfg, true
}

func (s *Service) sendEmail(subject, body string) {
	cfg, ok := s.smtpConfig()
	if !ok {
		return
	}

	addr := net.JoinHostPort(cfg.host, cfg.port)
	msg := buildMIME(cfg.from, cfg.to, subject, body)

	var auth smtp.Auth
	if cfg.user != "" {
		auth = smtp.PlainAuth("", cfg.user, cfg.password, cfg.host)
	}

	var err error
	if cfg.port == "465" {
		// SMTPS — implicit TLS
		tlsCfg := &tls.Config{ServerName: cfg.host, MinVersion: tls.VersionTLS12}
		conn, dialErr := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
		if dialErr != nil {
			logWarn("smtp dial failed", dialErr)
			return
		}
		defer conn.Close()
		c, clientErr := smtp.NewClient(conn, cfg.host)
		if clientErr != nil {
			logWarn("smtp client failed", clientErr)
			return
		}
		defer c.Quit() //nolint:errcheck
		if auth != nil {
			if err = c.Auth(auth); err != nil {
				logWarn("smtp auth failed", err)
				return
			}
		}
		if err = c.Mail(cfg.from); err == nil {
			err = c.Rcpt(cfg.to)
		}
		if err == nil {
			w, wErr := c.Data()
			if wErr != nil {
				logWarn("smtp data failed", wErr)
				return
			}
			_, err = fmt.Fprint(w, msg)
			if err == nil {
				err = w.Close()
			}
		}
	} else {
		// STARTTLS (port 587) or plain (port 25)
		err = smtp.SendMail(addr, auth, cfg.from, strings.Split(cfg.to, ","), []byte(msg))
	}
	if err != nil {
		logWarn("smtp send failed", err)
	}
}

func buildMIME(from, to, subject, body string) string {
	return fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		from, to, subject, body,
	)
}
