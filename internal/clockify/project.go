package clockify

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Project DTO
type Project struct {
	WorkspaceID string `json:"workspaceId"`

	ID    string `json:"id"`
	Name  string `json:"name"`
	Note  string `json:"note"`
	Color string `json:"color"`

	ClientID   string `json:"clientId"`
	ClientName string `json:"clientName"`

	HourlyRate Rate  `json:"hourlyRate"`
	CostRate   *Rate `json:"costRate"`
	Billable   bool  `json:"billable"`

	TimeEstimate   TimeEstimate `json:"timeEstimate"`
	BudgetEstimate BaseEstimate `json:"budgetEstimate"`
	Duration       *Duration    `json:"duration"`

	Archived bool `json:"archived"`
	Template bool `json:"template"`
	Public   bool `json:"public"`
	Favorite bool `json:"favorite"`

	Memberships []Membership `json:"memberships"`

	// Hydrated indicates if the attributes CustomFields and Tasks are filled
	Hydrated     bool          `json:"-"`
	CustomFields []CustomField `json:"customFields,omitempty"`
	Tasks        []Task        `json:"tasks,omitempty"`
}

// Rate DTO
type Rate struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency,omitempty"`
}

// MembershipStatus possible Membership Status
type MembershipStatus string

// MembershipStatusPending membership is Pending
const MembershipStatusPending = MembershipStatus("PENDING")

// MembershipStatusActive membership is Active
const MembershipStatusActive = MembershipStatus("ACTIVE")

// MembershipStatusDeclined membership is Declined
const MembershipStatusDeclined = MembershipStatus("DECLINED")

// MembershipStatusInactive membership is Inactive
const MembershipStatusInactive = MembershipStatus("INACTIVE")

func (p Project) GetID() string   { return p.ID }
func (p Project) GetName() string { return p.Name }

// CustomField DTO
type CustomField struct {
	CustomFieldID string `json:"customFieldId"`
	Status        string `json:"status"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Value         string `json:"value"`
}

// EstimateType possible Estimate types
type EstimateType string

// EstimateTypeAuto estimate is Auto
const EstimateTypeAuto = EstimateType("AUTO")

// EstimateTypeManual estimate is Manual
const EstimateTypeManual = EstimateType("MANUAL")

// EstimateResetOption possible Estimate Reset Options
type EstimateResetOption string

// EstimateResetOptionMonthly estimate is Auto
const EstimateResetOptionMonthly = EstimateResetOption("MONTHLY")

// BaseEstimate DTO
type BaseEstimate struct {
	Type         EstimateType         `json:"type"`
	Active       bool                 `json:"active"`
	ResetOptions *EstimateResetOption `json:"resetOptions"`
}

// TimeEstimate DTO
type TimeEstimate struct {
	BaseEstimate
	Estimate           Duration `json:"estimate"`
	IncludeNonBillable bool     `json:"includeNonBillable"`
}

// Duration is a time presentation for parameters
type Duration struct {
	time.Duration
}

// MarshalJSON converts Duration correctly
func (d Duration) MarshalJSON() ([]byte, error) {
	return []byte("\"" + d.String() + "\""), nil
}

// UnmarshalJSON converts a JSON value to Duration correctly
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	dc, err := StringToDuration(s)
	if err != nil {
		return err
	}

	*d = Duration{dc}
	return err
}

func StringToDuration(s string) (time.Duration, error) {
	if len(s) < 4 {
		return 0, fmt.Errorf("duration %s is invalid", s)
	}

	var u, dc time.Duration
	var j, i int
	for ; i < len(s); i++ {
		switch s[i] {
		case 'P', 'T':
			j = i + 1
			continue
		case 'H':
			u = time.Hour
		case 'M':
			u = time.Minute
		case 'S':
			u = time.Second
		default:
			continue
		}

		v, err := strconv.Atoi(s[j:i])
		if err != nil {
			return 0, err
		}
		dc = dc + time.Duration(v)*u
		j = i + 1
	}

	return dc, nil
}

func (d Duration) String() string {
	s := d.Duration.String()
	i := strings.LastIndex(s, ".")
	if i > -1 {
		s = s[0:i] + "s"
	}

	return "PT" + strings.ToUpper(s)
}

func (dd Duration) HumanString() string {
	d := dd.Duration
	p := ""
	if d < 0 {
		p = "-"
		d = d * -1
	}

	return p + fmt.Sprintf("%d:%02d:%02d",
		int64(d.Hours()), int64(d.Minutes())%60, int64(d.Seconds())%60)
}

// Task DTO
type Task struct {
	AssigneeIDs  []string   `json:"assigneeIds"`
	UserGroupIDs []string   `json:"userGroupIds"`
	Estimate     *Duration  `json:"estimate"`
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	ProjectID    string     `json:"projectId"`
	Billable     bool       `json:"billable"`
	HourlyRate   *Rate      `json:"hourlyRate"`
	CostRate     *Rate      `json:"costRate"`
	Status       TaskStatus `json:"status"`
	Duration     *Duration  `json:"duration"`
	Favorite     bool       `json:"favorite"`
}

// TaskStatus task status
type TaskStatus string

// TaskStatusActive task is Active
const TaskStatusActive = TaskStatus("ACTIVE")

// TaskStatusDone task is Done
const TaskStatusDone = TaskStatus("DONE")

type Projects []*Project
type Tasks []*Task
type Tags []*Tag

// Tag DTO
type Tag struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	WorkspaceID string `json:"workspaceId"`
}

func (p Projects) FindById(id string) *Project {
	for _, item := range p {
		if item.ID == id {
			return item
		}
	}
	return nil
}

func (p Projects) FindByName(name string) *Project {
	for _, item := range p {
		if item.Name == name {
			return item
		}
	}
	return nil
}

func (t Tasks) FindById(id string) *Task {
	for _, item := range t {
		if item.ID == id {
			return item
		}
	}
	return nil
}

func (t Tasks) FindByName(name string) *Task {
	for _, item := range t {
		if item.Name == name {
			return item
		}
	}
	return nil
}

func (t Tags) FindById(id string) *Tag {
	for _, item := range t {
		if item.ID == id {
			return item
		}
	}
	return nil
}

func (t Tags) FindByName(name string) *Tag {
	for _, item := range t {
		if item.Name == name {
			return item
		}
	}
	return nil
}

func (c *Client) GetWorkspaceProjects(wid string) (Projects, error) {
	var projects Projects
	res, err := c.request("GET", fmt.Sprintf("/workspaces/%s/projects", wid), nil)
	if err != nil {
		return projects, err
	}
	err = json.Unmarshal(res, &projects)
	return projects, err
}

func (c *Client) GetWorkspaceTags(wid string) (Tags, error) {
	var tags Tags
	res, err := c.request("GET", fmt.Sprintf("/workspaces/%s/tags", wid), nil)
	if err != nil {
		return tags, err
	}
	err = json.Unmarshal(res, &tags)
	return tags, err
}
