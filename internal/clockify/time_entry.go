package clockify

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/Ak-Army/xlog"
)

const dateFormatISO8601 = "2006-01-02T15:04:05+00:00"

type TimeEntry struct {
	Billable     bool         `json:"billable"`
	Description  string       `json:"description"`
	ID           string       `json:"id"`
	IsLocked     bool         `json:"isLocked"`
	ProjectID    string       `json:"projectId"`
	TimeInterval TimeInterval `json:"timeInterval"`
	UserID       string       `json:"userId"`
	WorkspaceID  string       `json:"workspaceId"`
}

type TimeInterval struct {
	Duration string     `json:"duration"`
	End      *time.Time `json:"end"`
	Start    time.Time  `json:"start"`
}

type currentResponse struct {
	Data TimeEntry `json:"data"`
}

func (c *Client) GetCurrentTimeEntry(wid string, user string) (TimeEntry, error) {
	res, err := c.request("GET", fmt.Sprintf("/workspaces/%s/user/%s/time-entries?in-progress=true", wid, user), nil)
	if err != nil {
		return TimeEntry{}, err
	}
	var response []TimeEntry
	err = json.Unmarshal(res, &response)
	if err != nil {
		return TimeEntry{}, err
	}
	if len(response) == 0 {
		return TimeEntry{}, nil
	}
	return response[0], nil
}

func (c *Client) StartTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	update := &updateTimeEntryRequest{
		Billable:    timeEntry.Billable,
		Description: timeEntry.Description,
		ProjectID:   timeEntry.ProjectID,
		Start:       &DateTime{Time: timeEntry.TimeInterval.Start},
	}
	if timeEntry.TimeInterval.End != nil {
		update.End = &DateTime{Time: *timeEntry.TimeInterval.End}
	}
	res, err := c.request("POST", fmt.Sprintf("/workspaces/%s/time-entries", timeEntry.WorkspaceID), update)
	if err != nil {
		xlog.Errorf("Unable to start time entry: %#v", string(res), err)
		return TimeEntry{}, err
	}
	var response = TimeEntry{}
	err = json.Unmarshal(res, &response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response, nil
}

func (c *Client) StopTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	res, err := c.request("PATCH",
		fmt.Sprintf("/workspaces/%s/user/%s/time-entries", timeEntry.WorkspaceID, timeEntry.UserID),
		&updateTimeEntryRequest{
			End: &DateTime{Time: time.Now()},
		})

	if err != nil {
		return TimeEntry{}, err
	}
	var response = TimeEntry{}
	err = json.Unmarshal(res, &response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response, nil
}

func (c *Client) GetTimeEntries(wid string, user string, fromDate time.Time, toDate time.Time) ([]TimeEntry, error) {
	var response []TimeEntry
	endpoint := fmt.Sprintf("/workspaces/%s/user/%s/time-entries", wid, user)
	if !fromDate.IsZero() {
		endpoint += "?start=" + url.QueryEscape(fromDate.Format(dateFormatISO8601))
		if !toDate.IsZero() {
			endpoint += "&end=" + url.QueryEscape(toDate.Format(dateFormatISO8601))
		} else {
			endpoint += "&end=" + url.QueryEscape(fromDate.Add(24*time.Hour).Format(dateFormatISO8601))
		}
	}
	res, err := c.request("GET", endpoint, nil)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(res, &response)
	if err != nil {
		return response, err
	}
	return response, nil
}

// UpdateTimeEntryRequest to update a time entry
type updateTimeEntryRequest struct {
	Start        *DateTime          `json:"start,omitempty"`
	End          *DateTime          `json:"end,omitempty"`
	Billable     bool               `json:"billable,omitempty"`
	Description  string             `json:"description,omitempty"`
	ProjectID    string             `json:"projectId,omitempty"`
	TaskID       string             `json:"taskId,omitempty"`
	TagIDs       []string           `json:"tagIds,omitempty"`
	CustomFields []CustomFieldValue `json:"customFields,omitempty"`
}

// DateTime is a time presentation for parameters
type DateTime struct {
	time.Time
}

// MarshalJSON converts DateTime correctly
func (d DateTime) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote(d.String())), nil
}

func (d DateTime) String() string {
	return d.Time.UTC().Format("2006-01-02T15:04:05Z")
}

// CustomFieldValue DTO
type CustomFieldValue struct {
	CustomFieldID string `json:"customFieldId"`
	Status        string `json:"status"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Value         string `json:"value"`
}

func (c *Client) UpdateTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	update := &updateTimeEntryRequest{
		Billable:    timeEntry.Billable,
		Description: timeEntry.Description,
		ProjectID:   timeEntry.ProjectID,
		Start:       &DateTime{Time: timeEntry.TimeInterval.Start},
	}
	if timeEntry.TimeInterval.End != nil {
		update.End = &DateTime{Time: *timeEntry.TimeInterval.End}
	}
	res, err := c.request("PUT",
		fmt.Sprintf("/workspaces/%s/time-entries/%s", timeEntry.WorkspaceID, timeEntry.ID), update)

	if err != nil {
		xlog.Errorf("Unable to update time entry: %#v", res, err)
		return TimeEntry{}, err
	}
	var response = TimeEntry{}
	err = json.Unmarshal(res, &response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response, nil
}

func (timeEntry TimeEntry) DurationInSec() float64 {
	duration, err := strconv.ParseInt(timeEntry.TimeInterval.Duration, 10, 64)
	if duration <= 0 || err != nil {
		if timeEntry.TimeInterval.End != nil {
			return timeEntry.TimeInterval.End.Sub(timeEntry.TimeInterval.Start).Seconds()
		}
		return time.Now().Sub(timeEntry.TimeInterval.Start).Seconds()
	}
	return float64(duration)
}
