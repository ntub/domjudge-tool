package email

import (
	"errors"
	"os"
	"path/filepath"
)

// scaffoldSubject, scaffoldBodyTxt, scaffoldBodyHTML are the canonical
// example template payloads. All six fixed placeholders are present so
// the resulting directory passes Template.Validate.
const scaffoldSubject = `Your DOMjudge account on $server_host`

const scaffoldBodyTxt = `Hello $name ($affiliation),

Your DOMjudge account credentials:
  Username: $username
  Password: $password
  Email:    $email
  Server:   $server_host

Please log in at https://$server_host and change your password.`

const scaffoldBodyHTML = `<!DOCTYPE html>
<html>
<body>
<p>Hello <strong>$name</strong> ($affiliation),</p>
<p>Your DOMjudge account credentials:</p>
<ul>
  <li>Username: <code>$username</code></li>
  <li>Password: <code>$password</code></li>
  <li>Email: <code>$email</code></li>
  <li>Server: <a href="https://$server_host">$server_host</a></li>
</ul>
<p>Please log in and change your password.</p>
</body>
</html>`

// WriteTemplateScaffold writes subject.txt, body.txt, and body.html into
// dir. All three contain every fixed placeholder so the directory loads
// and validates immediately. If force is false and any of the three
// target files already exists, returns ErrTemplateDirExists without
// writing anything. If force is true, only the three named files are
// overwritten; other files in dir are untouched.
func WriteTemplateScaffold(dir string, force bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	targets := []struct {
		name    string
		content string
	}{
		{"subject.txt", scaffoldSubject},
		{"body.txt", scaffoldBodyTxt},
		{"body.html", scaffoldBodyHTML},
	}
	if !force {
		for _, f := range targets {
			p := filepath.Join(dir, f.name)
			if _, err := os.Stat(p); err == nil {
				return ErrTemplateDirExists
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	for _, f := range targets {
		p := filepath.Join(dir, f.name)
		if err := os.WriteFile(p, []byte(f.content), 0o644); err != nil {
			return err
		}
	}
	return nil
}
