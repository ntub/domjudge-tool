package v4

import "context"

// GeneralResource wraps the /version endpoint.
type GeneralResource struct{ c *Client }

// versionBody is the JSON shape returned by GET /api/v4/version.
type versionBody struct {
	APIVersion string `json:"api_version"`
}

// Version returns the api_version string from GET /api/v4/version.
func (r *GeneralResource) Version(ctx context.Context) (string, error) {
	var body versionBody
	if err := r.c.getJSON(ctx, "/version", &body); err != nil {
		return "", err
	}
	return body.APIVersion, nil
}
