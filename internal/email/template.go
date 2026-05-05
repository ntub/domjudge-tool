// Package email provides the templated credential mailer use case.
package email

import (
	"errors"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// Template holds the loaded and parsed templates for a single email batch.
type Template struct {
	Subject  string
	Body     string
	BodyHTML bool
}

// LoadTemplate loads subject.txt and a body template from dir.
// HTML is preferred: body.html wins over body.txt when both exist.
func LoadTemplate(dir string) (*Template, error) {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("email: %q is not a directory: %w", dir, err)
	}

	subjectPath := filepath.Join(dir, "subject.txt")
	subjectBytes, err := os.ReadFile(subjectPath)
	if err != nil {
		return nil, fmt.Errorf("email: subject.txt not found in %q: %w", dir, err)
	}

	// HTML preferred (matches Python glob ordering: body.html before body.txt).
	htmlPath := filepath.Join(dir, "body.html")
	txtPath := filepath.Join(dir, "body.txt")

	var body string
	var bodyHTML bool
	if bodyBytes, err := os.ReadFile(htmlPath); err == nil {
		body = string(bodyBytes)
		bodyHTML = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("email: read %q: %w", htmlPath, err)
	} else if bodyBytes, err := os.ReadFile(txtPath); err == nil {
		body = string(bodyBytes)
		bodyHTML = false
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("email: read %q: %w", txtPath, err)
	} else {
		return nil, fmt.Errorf("email: no body.html or body.txt found in %q", dir)
	}

	return &Template{
		Subject:  string(subjectBytes),
		Body:     body,
		BodyHTML: bodyHTML,
	}, nil
}

// fixedVars is the complete set of placeholder names accepted by
// SendUserAccounts. Templates may reference any subset; unknown names are
// an operator error caught by Validate.
var fixedVars = map[string]bool{
	"username": true, "name": true, "email": true,
	"affiliation": true, "password": true, "server_host": true,
}

// Validate returns an error if the template's subject or body contains any
// $placeholder not in the fixed variable set (username, name, email,
// affiliation, password, server_host).
func (t *Template) Validate() error {
	if bad := collectUnknown(t.Subject); len(bad) > 0 {
		return fmt.Errorf("email: unknown placeholder(s) in subject: %v", bad)
	}
	if bad := collectUnknown(t.Body); len(bad) > 0 {
		return fmt.Errorf("email: unknown placeholder(s) in body: %v", bad)
	}
	return nil
}

// collectUnknown uses os.Expand to walk all $var / ${var} occurrences in s
// and returns those absent from fixedVars.
func collectUnknown(s string) []string {
	seen := make(map[string]bool)
	var unknown []string
	os.Expand(s, func(key string) string {
		if !fixedVars[key] && !seen[key] {
			seen[key] = true
			unknown = append(unknown, key)
		}
		return ""
	})
	return unknown
}

// Render applies vars to t using os.Expand ($var / ${var}) semantics.
// Unknown keys expand to "". Returns Q-encoded subject + raw body.
func (t *Template) Render(vars map[string]string) (subject, body string) {
	mapper := func(key string) string {
		if v, ok := vars[key]; ok {
			return v
		}
		return ""
	}
	rawSubject := os.Expand(t.Subject, mapper)
	subject = mime.QEncoding.Encode("UTF-8", strings.TrimRight(rawSubject, "\n"))
	body = os.Expand(t.Body, mapper)
	return subject, body
}
