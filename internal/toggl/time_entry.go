package toggl

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
	ID          int64      `json:"id,omitempty"`
	Description string     `json:"description"`
	WID         int64      `json:"wid,omitempty"`
	PID         int64      `json:"pid,omitempty"`
	TID         int64      `json:"tid,omitempty"`
	Billable    bool       `json:"billable,omitempty"`
	Start       time.Time  `json:"start,omitempty"`
	Stop        *time.Time `json:"stop,omitempty"`
	Duration    int64      `json:"duration,omitempty"`
	CreatedWith string     `json:"created_with"`
	Tags        []string   `json:"tags,omitempty"`
	DurOnly     bool       `json:"duronly,omitempty"`
	At          string     `json:"at,omitempty"`
}

type currentResponse struct {
	Data TimeEntry `json:"data"`
}

func (c *Client) GetCurrentTimeEntry() (TimeEntry, error) {
	res, err := c.request("GET", "/me/time_entries/current", nil)
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

func (c *Client) StartTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	res, err := c.request("POST", fmt.Sprintf("/workspaces/%d/time_entries", timeEntry.WID), timeEntry)
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
		fmt.Sprintf("/workspaces/%d/time_entries/%d/stop", timeEntry.WID, timeEntry.ID), nil)

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

func (c *Client) GetTimeEntry(id int) (TimeEntry, error) {
	idString := strconv.Itoa(id)
	res, err := c.request("GET", "/me/time_entries/"+idString, nil)
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

func (c *Client) GetTimeEntries(fromDate time.Time, toDate time.Time) ([]TimeEntry, error) {
	var response []TimeEntry
	endpoint := "/me/time_entries"
	if !fromDate.IsZero() {
		endpoint += "?start_date=" + url.QueryEscape(fromDate.Format(dateFormatISO8601))
		if !toDate.IsZero() {
			endpoint += "&end_date=" + url.QueryEscape(toDate.Format(dateFormatISO8601))
		} else {
			endpoint += "&end_date=" + url.QueryEscape(fromDate.Add(24*time.Hour).Format(dateFormatISO8601))
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

func (c *Client) UpdateTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	res, err := c.request("PUT",
		fmt.Sprintf("/workspaces/%d/time_entries/%d", timeEntry.WID, timeEntry.ID), timeEntry)

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
	var timeDur time.Duration
	if timeEntry.Duration < 0 {
		timeDur = time.Since(timeEntry.Start)
	} else {
		timeDur = time.Duration(timeEntry.Duration) * time.Second
	}
	return timeDur.Seconds()
}
