package handler_test

import (
	"html"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var handlerAttributePattern = regexp.MustCompile(`([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+)))?`)

func handlerTag(t *testing.T, body, tag, attribute, value string) map[string]string {
	t.Helper()
	pattern := regexp.MustCompile(`<` + regexp.QuoteMeta(tag) + `\b([^>]*)>`)
	for _, match := range pattern.FindAllStringSubmatch(body, -1) {
		attrs := make(map[string]string)
		for _, attr := range handlerAttributePattern.FindAllStringSubmatch(match[1], -1) {
			attrs[attr[1]] = html.UnescapeString(attr[2] + attr[3] + attr[4])
		}
		if got, ok := attrs[attribute]; ok && got == value {
			return attrs
		}
	}
	t.Fatalf("response missing <%s> with %s=%q", tag, attribute, value)
	return nil
}

func assertOperationRunPage(t *testing.T, rec *httptest.ResponseRecorder, streamURL, resultPrefix string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("run page status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	handlerTag(t, body, "div", "data-init", "@get('"+streamURL+"')")
	bar := handlerTag(t, body, "progress", "id", "import-progress")
	if bar["value"] != "0" {
		t.Errorf("initial progress value = %q, want 0", bar["value"])
	}
	if max, err := strconv.Atoi(bar["max"]); err != nil || max <= 0 {
		t.Errorf("progress max = %q, want a positive total", bar["max"])
	}
	count := handlerTag(t, body, "span", "id", "import-count")
	if count["aria-live"] != "polite" {
		t.Errorf("progress announcement = %q, want polite", count["aria-live"])
	}
	result := handlerTag(t, body, "div", "id", resultPrefix+"-result")
	if result["role"] != "status" {
		t.Errorf("result role = %q, want status", result["role"])
	}
}

func assertEmailRedirect(t *testing.T, rec *httptest.ResponseRecorder, path string, wantError bool) {
	t.Helper()
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("submit status = %d, want 303; body=%s", rec.Code, rec.Body.String())
	}
	loc, err := rec.Result().Location()
	if err != nil {
		t.Fatalf("redirect location: %v", err)
	}
	if loc.Path != path {
		t.Errorf("redirect path = %q, want %q", loc.Path, path)
	}
	if got := loc.Query().Get("kind") == "error"; got != wantError {
		t.Errorf("redirect error classification = %t, want %t; location=%s", got, wantError, loc)
	}
	if strings.TrimSpace(loc.Query().Get("flash")) == "" {
		t.Error("redirect must carry feedback for the submitted operation")
	}
}

func assertOperationStreamResult(t *testing.T, rec *httptest.ResponseRecorder, resultPrefix, classification string, wantProgress bool) {
	t.Helper()
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("stream status/content type = %d/%q", rec.Code, rec.Header().Get("Content-Type"))
	}
	patches := operationElementPatches(t, rec.Body.String())
	if len(patches) == 0 {
		t.Fatal("stream omitted the operation result")
	}
	alert := handlerTag(t, patches[len(patches)-1], "div", "id", resultPrefix+"-result")
	if alert["role"] != "status" || !strings.Contains(" "+alert["class"]+" ", " "+classification+" ") {
		t.Errorf("stream result semantics = %+v, want role=status and %s", alert, classification)
	}
	progressPatches := 0
	for _, patch := range patches[:len(patches)-1] {
		if !strings.Contains(patch, "<progress") {
			continue
		}
		progressPatches++
		handlerTag(t, patch, "progress", "id", "import-progress")
		count := handlerTag(t, patch, "span", "id", "import-count")
		if count["aria-live"] != "polite" {
			t.Errorf("stream count lost announcement semantics: %+v", count)
		}
	}
	if wantProgress && progressPatches == 0 {
		t.Error("stream omitted progress updates")
	}
}

func operationElementPatches(t *testing.T, body string) []string {
	t.Helper()
	var patches []string
	for _, event := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(event) == "" {
			continue
		}
		var eventType string
		var elements []string
		for _, line := range strings.Split(event, "\n") {
			if strings.HasPrefix(line, "event:") {
				eventType = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			}
			if strings.HasPrefix(line, "data: elements ") {
				elements = append(elements, strings.TrimPrefix(line, "data: elements "))
			}
		}
		if eventType != "datastar-patch-elements" {
			t.Fatalf("SSE event = %q, want datastar-patch-elements; frame=%s", eventType, event)
		}
		if len(elements) == 0 {
			t.Fatalf("patch event has no element payload: %s", event)
		}
		patches = append(patches, strings.Join(elements, "\n"))
	}
	return patches
}
