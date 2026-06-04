package v4

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GeneralResource wraps the /version endpoint.
type GeneralResource struct{ c *Client }

// versionBody is the JSON shape returned by GET /api/v4/version.
type versionBody struct {
	APIVersion apiVersionString `json:"api_version"`
}

// Version returns the api_version string from GET /api/v4/version.
func (r *GeneralResource) Version(ctx context.Context) (string, error) {
	var body versionBody
	if err := r.c.getJSON(ctx, "/version", &body); err != nil {
		return "", err
	}
	return string(body.APIVersion), nil
}

type apiVersionString string

func (v *apiVersionString) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	if s == "null" {
		*v = ""
		return nil
	}

	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*v = apiVersionString(str)
		return nil
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("api_version must be a string or integer: %w", err)
	}
	*v = apiVersionString(strconv.FormatInt(n, 10))
	return nil
}
