// Package web defines the version-agnostic Gateway contract that drives
// the DOMjudge HTML UI, plus shared parser helpers and sentinel errors
// that every implementation (v7 in Phase 1, v8 in Phase 2) returns.
//
// Lifecycle: callers MUST call Login(ctx) before any other method on a
// Gateway. Phase 1 implementations do not auto-relogin.
package web

import (
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ParseFormFields builds a url.Values snapshot of every named <input>,
// <textarea>, and <select> element under the document. The shape
// matches what a browser would post if the form were submitted as-is,
// so a caller that overlays a few overrides and POSTs the result
// preserves all existing state (multi-select choices, textarea
// content, hidden tokens) rather than silently clearing it.
//
// Differences from the Python reference (services/web/base.py
// _get_input_fields):
//
//   - Python read only <input> + <select>, took the first selected
//     <option> per <select>, and returned an explicit None for
//     "no selected option". This Go port adds <textarea> support,
//     emits ALL selected options for multi-selects, and represents
//     "no selected option" as key-absent. The first two changes fix
//     real Python data-loss bugs surfaced by the Codex adversarial
//     review; the third aligns with url.Values semantics so callers
//     no longer need an explicit nil-drop step.
//
// Inputs without a `name` attribute are silently dropped (matches the
// Python `data.pop(None, None)` at base.py:20).
func ParseFormFields(r io.Reader) (url.Values, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("web: parse form fields: %w", err)
	}
	out := url.Values{}

	doc.Find("input").Each(func(_ int, sel *goquery.Selection) {
		name, ok := sel.Attr("name")
		if !ok || name == "" {
			return
		}
		typ := strings.ToLower(sel.AttrOr("type", "text"))
		val, hasValue := sel.Attr("value")
		switch typ {
		case "checkbox", "radio":
			if _, checked := sel.Attr("checked"); !checked {
				return
			}
			if !hasValue {
				val = "on"
			}
			out.Add(name, val)
		default:
			out.Set(name, val)
		}
	})

	doc.Find("textarea").Each(func(_ int, sel *goquery.Selection) {
		name, ok := sel.Attr("name")
		if !ok || name == "" {
			return
		}
		out.Set(name, sel.Text())
	})

	doc.Find("select").Each(func(_ int, sel *goquery.Selection) {
		name, ok := sel.Attr("name")
		if !ok || name == "" {
			return
		}
		opts := sel.Find("option[selected]")
		if opts.Length() == 0 {
			// Match url.Values "key absent" semantics — a caller that
			// wants to drop the key from the POST simply doesn't add
			// it back via overrides.
			return
		}
		opts.Each(func(_ int, opt *goquery.Selection) {
			v, _ := opt.Attr("value")
			out.Add(name, v)
		})
	})

	return out, nil
}
