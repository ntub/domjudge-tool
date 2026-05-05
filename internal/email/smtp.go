package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// Sender is the narrow interface for delivering a pre-built MIME message.
// *smtpSender satisfies this in production; tests substitute a stub.
type Sender interface {
	// Send delivers msg (complete RFC 5322 message bytes) to recipients.
	// ctx cancellation is honoured: a cancelled ctx interrupts any blocking
	// SMTP operation and causes Send to return ctx.Err() (wrapped).
	Send(ctx context.Context, from string, to []string, msg []byte) error
	// Close shuts down the SMTP connection (QUIT; fallback CLOSE on error).
	Close() error
}

// DialOptions configures a Dial call.
type DialOptions struct {
	Host     string
	Port     int
	UseSSL   bool
	Timeout  time.Duration // 0 → no dial timeout
	Username string        // "" → skip AUTH
	Password string        // "" → skip AUTH
	// TLSConfig overrides the default TLS configuration when UseSSL is
	// true. When nil, Dial uses &tls.Config{ServerName: Host}. Tests
	// inject a config carrying RootCAs for self-signed certificates;
	// production callers normally leave this nil.
	TLSConfig *tls.Config
}

// plainAuth implements smtp.Auth for AUTH PLAIN without the TLS/localhost
// restriction imposed by smtp.PlainAuth. Python smtplib.SMTP.login has no
// equivalent restriction; this preserves Python parity for plain-SMTP+AUTH.
type plainAuth struct {
	username, password string
}

func (a plainAuth) Start(_ *smtp.ServerInfo) (mechanism string, response []byte, err error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("email: unexpected server challenge in AUTH PLAIN")
	}
	return nil, nil
}

// smtpSender wraps *smtp.Client to satisfy the Sender interface.
// conn is the underlying network connection used to set deadlines for
// context-driven cancellation.
type smtpSender struct {
	client *smtp.Client
	conn   net.Conn
}

// Dial opens an SMTP connection and returns a Sender.
//
// SSL path (UseSSL=true): tls.Dialer.DialContext + smtp.NewClient.
// Plain path (UseSSL=false): net.Dialer.DialContext + smtp.NewClient.
// Both paths: client.Hello("localhost") (EHLO).
// STARTTLS not implemented (mirrors Python commented-out starttls()).
// AUTH performed only when both Username and Password are non-empty.
func Dial(ctx context.Context, opts DialOptions) (Sender, error) {
	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	dialer := &net.Dialer{Timeout: opts.Timeout}

	var client *smtp.Client
	var rawConn net.Conn
	if opts.UseSSL {
		tlsCfg := opts.TLSConfig
		if tlsCfg == nil {
			tlsCfg = &tls.Config{ServerName: opts.Host}
		}
		tlsDialer := tls.Dialer{NetDialer: dialer, Config: tlsCfg}
		conn, err := tlsDialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, wrapContextErr(ctx, fmt.Sprintf("email: dial TLS %s", addr), err)
		}
		c, err := smtp.NewClient(conn, opts.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("email: smtp client: %w", err)
		}
		client, rawConn = c, conn
	} else {
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, wrapContextErr(ctx, fmt.Sprintf("email: dial %s", addr), err)
		}
		c, err := smtp.NewClient(conn, opts.Host)
		if err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("email: smtp client: %w", err)
		}
		client, rawConn = c, conn
	}

	if err := client.Hello("localhost"); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("email: ehlo: %w", err)
	}

	// STARTTLS not implemented (mirrors Python # self.connection.starttls())

	if opts.Username != "" && opts.Password != "" {
		if err := client.Auth(plainAuth{username: opts.Username, password: opts.Password}); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("email: auth: %w", err)
		}
	}

	return &smtpSender{client: client, conn: rawConn}, nil
}

// Send issues MAIL FROM, RCPT TO for each recipient, then DATA with msg.
// When ctx is cancelled, a goroutine sets a past deadline on the underlying
// connection so the in-flight SMTP operation returns immediately.
func (s *smtpSender) Send(ctx context.Context, from string, to []string, msg []byte) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("email: send: %w", err)
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			s.conn.SetDeadline(time.Now()) //nolint:errcheck // best-effort cancellation wake-up
		case <-done:
		}
	}()

	if err := s.client.Mail(from); err != nil {
		return wrapContextErr(ctx, "email: MAIL FROM", err)
	}
	for _, addr := range to {
		if err := s.client.Rcpt(addr); err != nil {
			return wrapContextErr(ctx, fmt.Sprintf("email: RCPT TO %s", addr), err)
		}
	}
	wc, err := s.client.Data()
	if err != nil {
		return wrapContextErr(ctx, "email: DATA", err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return wrapContextErr(ctx, "email: write message", err)
	}
	if err := wc.Close(); err != nil {
		return wrapContextErr(ctx, "email: close DATA", err)
	}
	return nil
}

// Close attempts a graceful QUIT; on error it falls back to a hard close,
// mirroring Python smtplib.SMTP.close()'s try/finally.
func (s *smtpSender) Close() error {
	err := s.client.Quit()
	if err != nil {
		_ = s.client.Close()
		return fmt.Errorf("email: quit: %w", err)
	}
	return nil
}

func wrapContextErr(ctx context.Context, msg string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%s: %w: %w", msg, ctxErr, err)
	}
	return fmt.Errorf("%s: %w", msg, err)
}
