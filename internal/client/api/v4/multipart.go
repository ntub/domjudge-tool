package v4

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
)

type multipartField interface {
	writeMultipart(name string, w *multipart.Writer) error
}

type multipartValue struct {
	value string
}

func (v multipartValue) writeMultipart(name string, w *multipart.Writer) error {
	if err := w.WriteField(name, v.value); err != nil {
		return fmt.Errorf("write field %q: %w", name, err)
	}
	return nil
}

type multipartFile struct {
	filename    string
	contentType string
	reader      io.Reader
}

func (f multipartFile) writeMultipart(name string, w *multipart.Writer) error {
	if f.reader == nil {
		return fmt.Errorf("%w: multipart file %q reader is nil", ErrInvalidArgument, name)
	}
	filename := f.filename
	if filename == "" {
		filename = "upload"
	}
	contentType := f.contentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf("form-data; name=%q; filename=%q", name, filename))
	header.Set("Content-Type", contentType)
	part, err := w.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create file part %q: %w", name, err)
	}
	if _, err := io.Copy(part, f.reader); err != nil {
		return fmt.Errorf("copy file part %q: %w", name, err)
	}
	return nil
}

func (c *Client) postMultipart(ctx context.Context, path string, fields map[string]multipartField, out interface{}) error {
	if err := validateMultipartFields(fields); err != nil {
		return err
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	contentType := writer.FormDataContentType()
	writeErr := make(chan error, 1)

	go func() {
		err := writeMultipartFields(writer, fields)
		if closeErr := writer.Close(); err == nil {
			err = closeErr
		}
		_ = pw.CloseWithError(err)
		writeErr <- err
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		<-writeErr //nolint:errcheck // draining background writer on early abort
		return fmt.Errorf("build multipart request: %w", err)
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.postHTTP.Do(req)
	if err != nil {
		_ = pr.CloseWithError(err)
		<-writeErr //nolint:errcheck // draining background writer on early abort
		return fmt.Errorf("post multipart: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	fieldErr := <-writeErr
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return multipartHTTPError(resp.StatusCode, body, c.base+path)
	}
	if fieldErr != nil {
		return fmt.Errorf("write multipart: %w", fieldErr)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func validateMultipartFields(fields map[string]multipartField) error {
	if len(fields) == 0 {
		return fmt.Errorf("%w: multipart fields must not be empty", ErrInvalidArgument)
	}
	for name, field := range fields {
		switch f := field.(type) {
		case nil:
			return fmt.Errorf("%w: multipart field %q is nil", ErrInvalidArgument, name)
		case multipartFile:
			if f.reader == nil {
				return fmt.Errorf("%w: multipart file %q reader is nil", ErrInvalidArgument, name)
			}
		case *multipartFile:
			if f == nil || f.reader == nil {
				return fmt.Errorf("%w: multipart file %q reader is nil", ErrInvalidArgument, name)
			}
		}
	}
	return nil
}

func writeMultipartFields(w *multipart.Writer, fields map[string]multipartField) error {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := fields[name].writeMultipart(name, w); err != nil {
			return err
		}
	}
	return nil
}

func multipartHTTPError(statusCode int, body []byte, rawURL string) error {
	if statusCode >= 400 && statusCode < 500 {
		if details, ok := importRejectionDetails(body); ok {
			return &ImportRejectedError{StatusCode: statusCode, Body: body, URL: rawURL, Details: details}
		}
	}
	return &HTTPError{StatusCode: statusCode, Body: body, URL: rawURL}
}

func importRejectionDetails(body []byte) ([]string, bool) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 || body[0] != '[' {
		return nil, false
	}

	var rawStrings []string
	if err := json.Unmarshal(body, &rawStrings); err == nil {
		return rawStrings, true
	}

	var objects []map[string]interface{}
	if err := json.Unmarshal(body, &objects); err != nil {
		return nil, false
	}
	var messages []string
	for _, obj := range objects {
		found := false
		for _, key := range []string{"message", "error", "detail"} {
			if value, ok := obj[key].(string); ok && value != "" {
				messages = append(messages, value)
				found = true
				break
			}
		}
		if found {
			continue
		}
		raw, err := json.Marshal(obj)
		if err != nil {
			return nil, false
		}
		messages = append(messages, string(raw))
	}
	return messages, true
}
