package web

import (
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestParseFormFields_AllInputTypesIncluded(t *testing.T) {
	html := `
<form>
  <input type="hidden" name="_csrf_token" value="ABC">
  <input type="text" name="_username" value="alice">
  <input type="password" name="_password" value="">
  <input type="submit" name="login" value="Sign in">
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	want := url.Values{
		"_csrf_token": []string{"ABC"},
		"_username":   []string{"alice"},
		"_password":   []string{""},
		"login":       []string{"Sign in"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseFormFields_NamelessInputsDropped(t *testing.T) {
	html := `
<form>
  <input type="hidden" value="orphan">
  <input type="text" name="kept" value="ok">
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	if _, present := got[""]; present {
		t.Errorf("empty-name key should be dropped")
	}
	if got.Get("kept") != "ok" {
		t.Errorf("kept = %q, want ok", got.Get("kept"))
	}
}

func TestParseFormFields_CheckedInputsOnly(t *testing.T) {
	html := `
<form>
  <input type="checkbox" name="user[user_roles][]" value="1">
  <input type="checkbox" name="user[user_roles][]" value="3" checked="checked">
  <input type="checkbox" name="user[user_roles][]" value="11">
  <input type="checkbox" name="implicit" checked="checked">
  <input type="radio" name="enabled" value="1" checked="checked">
  <input type="radio" name="enabled" value="0">
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	if want := []string{"3"}; !reflect.DeepEqual(got["user[user_roles][]"], want) {
		t.Errorf("user[user_roles][] = %v, want %v", got["user[user_roles][]"], want)
	}
	if got.Get("implicit") != "on" {
		t.Errorf("implicit checkbox = %q, want on", got.Get("implicit"))
	}
	if got.Get("enabled") != "1" {
		t.Errorf("enabled = %q, want 1", got.Get("enabled"))
	}
}

func TestParseFormFields_SelectsTakeSelectedOption(t *testing.T) {
	html := `
<form>
  <select name="team[category]">
    <option value="1">First</option>
    <option value="2" selected="selected">Second</option>
    <option value="3">Third</option>
  </select>
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	if got.Get("team[category]") != "2" {
		t.Errorf("team[category] = %q, want 2", got.Get("team[category]"))
	}
}

// TestParseFormFields_SelectsWithNoSelectedOptionAreAbsent locks the
// expected behaviour: a <select> with no selected option contributes
// no entry to the result. Callers that previously relied on Python's
// "key present with None value" pattern should check key-absence
// instead.
func TestParseFormFields_SelectsWithNoSelectedOptionAreAbsent(t *testing.T) {
	html := `
<form>
  <select name="team[contests][]">
    <option value="1">A</option>
    <option value="2">B</option>
  </select>
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	if _, present := got["team[contests][]"]; present {
		t.Errorf("team[contests][] should be absent (no selected option), got %v", got["team[contests][]"])
	}
}

// TestParseFormFields_MultiSelectAllSelectedValuesPreserved locks the
// fix for Codex finding #3: a multi-select with several selected
// options contributes EVERY selected value to the result, so a
// subsequent form replay does not silently drop the team's existing
// associations.
func TestParseFormFields_MultiSelectAllSelectedValuesPreserved(t *testing.T) {
	html := `
<form>
  <select name="team[contests][]" multiple="multiple">
    <option value="1">A</option>
    <option value="2" selected="selected">B</option>
    <option value="3" selected="selected">C</option>
    <option value="5" selected="selected">E</option>
  </select>
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	wantValues := []string{"2", "3", "5"}
	gotValues := got["team[contests][]"]
	if !reflect.DeepEqual(gotValues, wantValues) {
		t.Errorf("team[contests][] = %v, want %v", gotValues, wantValues)
	}
}

// TestParseFormFields_TextareaIncluded locks the fix for Codex
// finding #3: <textarea> elements (e.g. team[members], team[comments])
// must contribute their inner text to the form snapshot so a re-POST
// preserves the existing value rather than silently clearing it.
func TestParseFormFields_TextareaIncluded(t *testing.T) {
	html := `
<form>
  <textarea name="team[members]">Alice
Bob
Charlie</textarea>
  <textarea name="team[comments]">  trimmed leading/trailing whitespace inside textarea is preserved  </textarea>
  <textarea>orphan-textarea-no-name</textarea>
</form>`
	got, err := ParseFormFields(strings.NewReader(html))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	if want, gotVal := "Alice\nBob\nCharlie", got.Get("team[members]"); gotVal != want {
		t.Errorf("team[members] = %q, want %q", gotVal, want)
	}
	if want, gotVal := "  trimmed leading/trailing whitespace inside textarea is preserved  ", got.Get("team[comments]"); gotVal != want {
		t.Errorf("team[comments] = %q, want %q", gotVal, want)
	}
	if _, present := got[""]; present {
		t.Errorf("nameless textarea must be dropped")
	}
}

func TestParseFormFields_EmptyHTML(t *testing.T) {
	got, err := ParseFormFields(strings.NewReader(""))
	if err != nil {
		t.Fatalf("ParseFormFields: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d keys, want 0", len(got))
	}
}

func TestParseFormFields_MalformedHTMLToleratedByGoquery(t *testing.T) {
	html := `<form><input name="x" value="y"<broken><select name="y">`
	if _, err := ParseFormFields(strings.NewReader(html)); err != nil {
		t.Fatalf("malformed HTML returned err %v; goquery tolerates these", err)
	}
}
