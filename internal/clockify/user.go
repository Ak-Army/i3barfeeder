package clockify

import (
	"encoding/json"
)

type User struct {
	ID               string       `json:"id"`
	ActiveWorkspace  string       `json:"activeWorkspace"`
	DefaultWorkspace string       `json:"defaultWorkspace"`
	Email            string       `json:"email"`
	Memberships      []Membership `json:"memberships"`
	Name             string       `json:"name"`
	ProfilePicture   string       `json:"profilePicture"`
	/*Settings         UserSettings `json:"settings"`
	Status           UserStatus   `json:"status"`
	Roles            *[]Role      `json:"roles"`*/
}

// Membership DTO
type Membership struct {
	HourlyRate *Rate            `json:"hourlyRate"`
	CostRate   *Rate            `json:"costRate"`
	Status     MembershipStatus `json:"membershipStatus"`
	Type       string           `json:"membershipType"`
	TargetID   string           `json:"targetId"`
	UserID     string           `json:"userId"`
}

func (c *Client) User() (*User, error) {
	var response *User
	res, err := c.request("GET", "/user", nil)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(res, &response)

	return response, err
}
