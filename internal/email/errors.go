package email

import "errors"

// ErrTemplateDirExists is returned by WriteTemplateScaffold when force is
// false and one or more of the three target files already exist.
var ErrTemplateDirExists = errors.New("email: template directory already contains target files")
