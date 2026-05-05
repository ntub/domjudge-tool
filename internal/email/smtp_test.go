package email_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ntub/domjudge-tool/internal/email"
)

// fakeSMTPServer starts a one-shot TCP listener on 127.0.0.1 and dispatches
// the first accepted connection to handler in a goroutine. The listener is
// registered with t.Cleanup. Returns the bound host and port for Dial.
func fakeSMTPServer(t *testing.T, handler func(conn net.Conn)) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.IP.String(), addr.Port
}

// fakeTLSSMTPServer is the TLS variant of fakeSMTPServer, wrapping the
// listener with the supplied certificate. Returns the bound port; host is
// always "127.0.0.1" so the cert IP SAN matches.
func fakeTLSSMTPServer(t *testing.T, cert tls.Certificate, handler func(conn net.Conn)) int {
	t.Helper()
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{cert}})
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		handler(conn)
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// generateSelfSignedCert returns a TLS certificate (for the listener) and
// the parsed *x509.Certificate (for adding to a RootCAs pool). The cert
// has a 127.0.0.1 IP SAN so ServerName="127.0.0.1" passes verification.
func generateSelfSignedCert(t *testing.T) (tls.Certificate, *x509.Certificate) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	parsed, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}, parsed
}

// plainSMTPHandler scripts the minimum greeting + EHLO + QUIT sequence.
func plainSMTPHandler(conn net.Conn) {
	fmt.Fprintf(conn, "220 test ready\r\n")
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "EHLO"):
			fmt.Fprintf(conn, "250 OK\r\n")
		case strings.HasPrefix(line, "QUIT"):
			fmt.Fprintf(conn, "221 Bye\r\n")
			return
		}
	}
}

func TestDial_PlainConnects(t *testing.T) {
	host, port := fakeSMTPServer(t, plainSMTPHandler)

	sender, err := email.Dial(context.Background(), email.DialOptions{
		Host:    host,
		Port:    port,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if sender == nil {
		t.Fatal("Dial returned nil Sender")
	}
	if err := sender.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestDial_WithAuth(t *testing.T) {
	var gotAuth string
	host, port := fakeSMTPServer(t, func(conn net.Conn) {
		fmt.Fprintf(conn, "220 test ready\r\n")
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "EHLO"):
				// Multi-line EHLO response advertising AUTH PLAIN. Last
				// line MUST use a space after the code (per RFC 5321);
				// continuation lines use a hyphen.
				fmt.Fprintf(conn, "250-test\r\n")
				fmt.Fprintf(conn, "250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH PLAIN"):
				gotAuth = line
				fmt.Fprintf(conn, "235 2.7.0 OK\r\n")
			case strings.HasPrefix(line, "QUIT"):
				fmt.Fprintf(conn, "221 Bye\r\n")
				return
			}
		}
	})

	sender, err := email.Dial(context.Background(), email.DialOptions{
		Host:     host,
		Port:     port,
		Username: "u",
		Password: "p",
		Timeout:  2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if sender == nil {
		t.Fatal("Dial returned nil Sender")
	}
	if !strings.HasPrefix(gotAuth, "AUTH PLAIN ") {
		t.Errorf("server did not receive AUTH PLAIN; got %q", gotAuth)
	}
	if err := sender.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestSend_DeliversMessage(t *testing.T) {
	var (
		gotFrom string
		gotTo   []string
		gotBody []byte
	)
	done := make(chan struct{})
	host, port := fakeSMTPServer(t, func(conn net.Conn) {
		defer close(done)
		fmt.Fprintf(conn, "220 test ready\r\n")
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprintf(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "MAIL FROM:"):
				gotFrom = strings.TrimSuffix(strings.TrimPrefix(line, "MAIL FROM:<"), ">")
				fmt.Fprintf(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "RCPT TO:"):
				addr := strings.TrimSuffix(strings.TrimPrefix(line, "RCPT TO:<"), ">")
				gotTo = append(gotTo, addr)
				fmt.Fprintf(conn, "250 OK\r\n")
			case line == "DATA":
				fmt.Fprintf(conn, "354 Start mail input\r\n")
				// Read body bytes until the SMTP terminator "\r\n.\r\n".
				var buf bytes.Buffer
				const term = "\r\n.\r\n"
				for {
					b, err := reader.ReadByte()
					if err != nil {
						return
					}
					buf.WriteByte(b)
					if buf.Len() >= len(term) && bytes.HasSuffix(buf.Bytes(), []byte(term)) {
						break
					}
				}
				gotBody = append([]byte(nil), buf.Bytes()[:buf.Len()-len(term)]...)
				fmt.Fprintf(conn, "250 OK\r\n")
			case line == "QUIT":
				fmt.Fprintf(conn, "221 Bye\r\n")
				return
			}
		}
	})

	sender, err := email.Dial(context.Background(), email.DialOptions{
		Host:    host,
		Port:    port,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	msg := []byte("Subject: hi\r\n\r\nbody bytes")
	if err := sender.Send(context.Background(), "from@example.com", []string{"a@example.com", "b@example.com"}, msg); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := sender.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	<-done // synchronise with handler completion before reading captures.

	if gotFrom != "from@example.com" {
		t.Errorf("MAIL FROM: got %q, want %q", gotFrom, "from@example.com")
	}
	wantTo := []string{"a@example.com", "b@example.com"}
	if !reflect.DeepEqual(gotTo, wantTo) {
		t.Errorf("RCPT TO: got %v, want %v", gotTo, wantTo)
	}
	if !bytes.Equal(gotBody, msg) {
		t.Errorf("DATA body: got %q, want %q", gotBody, msg)
	}
}

func TestSend_ContextCancelDuringSMTP(t *testing.T) {
	// The fake server handles EHLO/MAIL/RCPT normally but stalls after
	// receiving DATA — it never sends "354 Start mail input". The test
	// cancels ctx and expects Send to return (not block) within 2 s.
	stalled := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	host, port := fakeSMTPServer(t, func(conn net.Conn) {
		fmt.Fprintf(conn, "220 test ready\r\n")
		reader := bufio.NewReader(conn)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprintf(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "MAIL FROM"):
				fmt.Fprintf(conn, "250 OK\r\n")
			case strings.HasPrefix(line, "RCPT TO"):
				fmt.Fprintf(conn, "250 OK\r\n")
			case line == "DATA":
				close(stalled) // signal test that server is now stalling
				<-release      // hold until test cleanup
				return
			}
		}
	})

	sender, err := email.Dial(context.Background(), email.DialOptions{
		Host:    host,
		Port:    port,
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- sender.Send(ctx, "from@example.com", []string{"to@example.com"},
			[]byte("Subject: test\r\n\r\nbody"))
	}()

	<-stalled // wait until server is blocked on DATA response
	cancel()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("want error after ctx cancel, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Send did not return after ctx cancel within 2 s")
	}
}

func TestDial_SSLContextCancelDuringHandshake(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	accepted := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		close(accepted)
		<-release
		_ = conn.Close()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		sender, err := email.Dial(ctx, email.DialOptions{
			Host:    "127.0.0.1",
			Port:    ln.Addr().(*net.TCPAddr).Port,
			UseSSL:  true,
			Timeout: 5 * time.Second,
			TLSConfig: &tls.Config{
				ServerName: "127.0.0.1",
			},
		})
		if sender != nil {
			_ = sender.Close()
		}
		errCh <- err
	}()

	select {
	case <-accepted:
		cancel()
	case err := <-errCh:
		t.Fatalf("Dial returned before handshake was cancelled: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("TLS server did not accept connection")
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("want error after ctx cancel, got nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("errors.Is(err, context.Canceled) = false; err = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TLS Dial did not return after ctx cancel within 2 s")
	}
}

func TestDial_SSLConnects(t *testing.T) {
	cert, parsed := generateSelfSignedCert(t)
	port := fakeTLSSMTPServer(t, cert, plainSMTPHandler)

	pool := x509.NewCertPool()
	pool.AddCert(parsed)

	sender, err := email.Dial(context.Background(), email.DialOptions{
		Host:    "127.0.0.1",
		Port:    port,
		UseSSL:  true,
		Timeout: 2 * time.Second,
		TLSConfig: &tls.Config{
			ServerName: "127.0.0.1",
			RootCAs:    pool,
		},
	})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if sender == nil {
		t.Fatal("Dial returned nil Sender")
	}
	if err := sender.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}
