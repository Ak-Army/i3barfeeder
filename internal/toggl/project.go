package toggl

import (
	"encoding/json"
	"fmt"
)

type Project struct {
	Active        bool   `json:"active"`
	ActualHours   int    `json:"actual_hours"`
	At            string `json:"at"`
	AutoEstimates bool   `json:"auto_estimates"`
	Billable      bool   `json:"billable"`
	Color         string `json:"color"`
	CreatedAt     string `json:"created_at"`
	HexColor      string `json:"hex_color"`
	ID            int64    `json:"id"`
	IsPrivate     bool   `json:"is_private"`
	Name          string `json:"name"`
	Template      bool   `json:"template"`
	WID           int64    `json:"wid"`
	Tasks         Tasks
}

type Task struct {
	Name             string    `json:"name"`
	ID               int64       `json:"id"`
	WID              int64       `json:"wid"`
	PID              int64       `json:"pid"`
	Active           bool      `json:"active"`
	At               string  `json:"at"`
	EstimatedSeconds int       `json:"estimated_seconds"`
}

type Projects []*Project
type Tasks []*Task

func (p Projects) FindById(id int64) *Project {
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

func (t Tasks) FindById(id int64) *Task {
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

func (c *Client) GetWorkspaceProjects(wid int64) (Projects, error) {
	var projects Projects
	res, err := c.request("GET", fmt.Sprintf("/workspaces/%d/projects",wid), nil)
	if err != nil {
		return projects, err
	}
	err = json.Unmarshal(res, &projects)
	return projects, err
}

func (c *Client) GetProjectTasks(pid int64) (Tasks, error) {
	var tasks Tasks
	res, err := c.request("GET", fmt.Sprintf("/projects/%d/tasks", pid), nil)
	if err != nil {
		return tasks, err
	}
	err = json.Unmarshal(res, &tasks)
	return tasks, err
}
