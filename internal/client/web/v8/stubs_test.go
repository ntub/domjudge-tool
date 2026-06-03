package v8

import (
	"context"
	"errors"
	"testing"

	"github.com/ntub/domjudge-tool/internal/client/web"
	"github.com/ntub/domjudge-tool/internal/model"
)

func TestUnsupportedStubs_ReturnErrUnsupportedOnV8(t *testing.T) {
	c := &Client{}
	ctx := context.Background()

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "GetAffiliations", run: func() error {
			_, err := c.GetAffiliations(ctx)
			return err
		}},
		{name: "GetAffiliation", run: func() error {
			_, err := c.GetAffiliation(ctx, "TW")
			return err
		}},
		{name: "CreateAffiliation", run: func() error {
			_, err := c.CreateAffiliation(ctx, "TW", "Taiwan", "TWN")
			return err
		}},
		{name: "SetUserPassword", run: func() error {
			return c.SetUserPassword(ctx, "11", "secret", []int{1}, true)
		}},
		{name: "UpdateUserEmail", run: func() error {
			return c.UpdateUserEmail(ctx, "11", "user1@example.org")
		}},
		{name: "CreateTeamAndUser", run: func() error {
			_, _, err := c.CreateTeamAndUser(ctx, model.CreateUser{Username: "u"}, 1, 1, true)
			return err
		}},
		{name: "UpdateTeam", run: func() error {
			teamID := "9"
			_, _, err := c.UpdateTeam(ctx, model.User{Username: "u", TeamID: &teamID}, 1, 1, true)
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run()
			if !errors.Is(err, web.ErrUnsupportedOnV8) {
				t.Errorf("errors.Is(err, web.ErrUnsupportedOnV8) = false; err = %v", err)
			}
		})
	}
}
