// Package v8 implements web.Gateway against DOMjudge 8.3.1's
// authenticated /jury HTML UI.
//
// v8 implements only the 5 jury operations the v4 API cannot do;
// create/update/password/email/affiliation are intentionally unsupported
// here and route to the v4 REST API via the importer in P2-4. These
// stubs must never be on a default v8 code path once P2-4 lands.
package v8
