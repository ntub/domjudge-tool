package model

// User maps to `domjudge_tool_cli.models.user.User`.
//
// The field order matches the Pydantic class declaration so that
// `json.MarshalIndent` produces output that round-trips byte-identically
// against fixtures derived from Python output.
type User struct {
	LastLoginTime  *string  `json:"last_login_time"`
	FirstLoginTime *string  `json:"first_login_time"`
	Team           *string  `json:"team"`
	Roles          []string `json:"roles"`
	ID             string   `json:"id"`
	Username       string   `json:"username"`
	Name           string   `json:"name"`
	Email          *string  `json:"email"`
	LastIP         *string  `json:"last_ip"`
	IP             *string  `json:"ip"`
	Enabled        bool     `json:"enabled"`
	TeamID         *string  `json:"team_id"`
	Affiliation    *string  `json:"affiliation"`
	Password       *string  `json:"password"`
}

// CreateUser maps to `domjudge_tool_cli.models.user.CreateUser`. It is
// the payload used when provisioning a user through the web UI.
type CreateUser struct {
	Username    string  `json:"username"`
	Name        string  `json:"name"`
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	Affiliation *string `json:"affiliation"`
	IsExist     *bool   `json:"is_exist"`
}

// CreateUserOverrides lets callers tweak fields when converting a User
// into a CreateUser via [CreateUserFromUser]. Mirrors the **kwargs
// override semantics of the Python `CreateUser.from_user` classmethod —
// nil fields are left untouched.
type CreateUserOverrides struct {
	Name        *string
	Email       *string
	Password    *string
	Affiliation *string
	IsExist     *bool
}

// CreateUserFromUser builds a CreateUser from an existing User, defaulting
// IsExist to true. Optional override values replace the corresponding
// User-derived fields. Equivalent to Python `CreateUser.from_user(user)`.
func CreateUserFromUser(u User, override ...CreateUserOverrides) CreateUser {
	isExist := true
	cu := CreateUser{
		Username:    u.Username,
		Name:        u.Name,
		Email:       u.Email,
		Password:    u.Password,
		Affiliation: u.Affiliation,
		IsExist:     &isExist,
	}
	for _, o := range override {
		if o.Name != nil {
			cu.Name = *o.Name
		}
		if o.Email != nil {
			cu.Email = o.Email
		}
		if o.Password != nil {
			cu.Password = o.Password
		}
		if o.Affiliation != nil {
			cu.Affiliation = o.Affiliation
		}
		if o.IsExist != nil {
			cu.IsExist = o.IsExist
		}
	}
	return cu
}
