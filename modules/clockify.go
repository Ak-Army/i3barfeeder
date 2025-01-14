package modules

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ak-Army/timer"
	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"github.com/Ak-Army/i3barfeeder/internal/clockify"
)

func init() {
	gobar.AddModule("Clockify", func() gobar.ModuleInterface {
		return &Clockify{todayDuration: "00s"}
	})
}

type Clockify struct {
	sync.Mutex
	gobar.ModuleInterface
	ApiToken         string        `json:"apiToken"`
	TicketNames      []cticketName `json:"ticketNames"`
	tickets          []cticket
	currentTimeEntry clockify.TimeEntry
	updateTimeEntry  clockify.TimeEntry
	todayDuration    string
	currentName      int
	updateTimer      timer.Timer
	log              xlog.Logger
	projects         clockify.Projects
	clockifyClient   clockify.Client
	clockifyUser     *clockify.User
}

type cticketName struct {
	Name    string `json:"name"`
	TPId    string `json:"tpId"`
	Project string `json:"project"`
}

type cticket struct {
	name  string
	PID   string
	TagId string
}

func (m *Clockify) InitModule(config json.RawMessage, log xlog.Logger) error {
	m.log = log
	if err := json.Unmarshal(config, m); err != nil {
		return err
	}
	m.clockifyClient = clockify.NewClient(m.ApiToken)
	var err error
	m.clockifyUser, err = m.clockifyClient.User()
	m.log.Debugf("User %+v", m.clockifyUser)
	if err != nil {
		return err
	}
	if m.clockifyUser == nil {
		return errors.New("no user found")
	}
	m.calcRemainingTime()
	m.updateProjectsAndTasks()

	ticker := timer.NewTicker("clockifyTicker", 10*time.Second)
	go func() {
		for t := range ticker.C() {
			m.Lock()
			if m.updateTimeEntry.ID == "" {
				m.getCurrentTimeEntry()
			}
			if t.Minute() > 0 && t.Minute()%5 == 0 {
				m.calcRemainingTime()
				m.updateProjectsAndTasks()
			}
			m.Unlock()
		}
	}()
	m.updateTimer = timer.NewTimer("togglUpdateTimer", time.Second)
	go func() {
		for {
			select {
			case <-m.updateTimer.C():
				m.updateCurrentTimeEntry()
			}
		}
	}()
	return nil
}

func (m *Clockify) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if m.currentTimeEntry.ID != "" {
		prettyTime := fmt.Sprintf("%s / %s",
			prettyPrintDuration(int(m.currentTimeEntry.DurationInSec()), true),
			m.todayDuration)
		shortDesc := m.currentTimeEntry.Description
		if len(m.currentTimeEntry.Description) > 7 {
			shortDesc = m.currentTimeEntry.Description[0:7]
		}
		info.ShortText = fmt.Sprintf("%s - %s", shortDesc, prettyTime)
		info.FullText = fmt.Sprintf("%s - %s", m.currentTimeEntry.Description, prettyTime)
	} else {
		info.ShortText = fmt.Sprintf("%s", m.todayDuration)
		info.FullText = fmt.Sprintf("%s", info.ShortText)
	}
	return info
}

// {"name":"Toggl","instance":"id_0","button":5,"x":2991,"y":12}
func (m *Clockify) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	m.Lock()
	defer m.Unlock()
	m.currentTimeEntry, _ = m.clockifyClient.GetCurrentTimeEntry(m.clockifyUser.DefaultWorkspace, m.clockifyUser.ID)
	m.log.Infof("HandleClick %+v %#v", cm.Button, m.currentTimeEntry)
	m.updateTimer.SafeStop()
	m.updateTimeEntry = clockify.TimeEntry{}
	switch cm.Button {
	case 2: // middle button
		now := time.Now()
		from := now.AddDate(0, -1, 0)
		entries, err := m.clockifyClient.GetTimeEntries(m.clockifyUser.DefaultWorkspace, m.clockifyUser.ID, from, now)
		if err == nil {
			days := map[string]dayEntry{}
			var sortDay []string
			for date := from; date.Before(now); date = date.AddDate(0, 0, 1) {
				i := date.Format("2006-01-02")
				days[i] = dayEntry{
					Duration: 0,
					Date:     date,
				}
				sortDay = append(sortDay, i)
			}
			for _, timeEntry := range entries {
				if timeEntry.TimeInterval.Duration == "0" {
					continue
				}
				day := timeEntry.TimeInterval.Start.Format("2006-01-02")
				days[day] = dayEntry{
					Duration: days[day].Duration + int64(timeEntry.DurationInSec()),
					Date:     timeEntry.TimeInterval.Start,
				}
			}
			m.log.Debugf("Middle click %+v", sortDay)
			sort.Strings(sortDay)
			copyCmd := exec.Command("xclip", "-selection", "c")
			in, err := copyCmd.StdinPipe()
			if err == nil {
				err = copyCmd.Start()
				defer copyCmd.Wait()
				defer in.Close()
				if err == nil {
					var output []string
					for _, day := range sortDay {
						de := days[day]
						weekday := de.Date.Weekday()
						isWeekend := weekday == 0 || weekday == 6
						sum := de.Duration / 60
						if !isWeekend {
							sum = (de.Duration - 28800) / 60
						}
						in.Write([]byte(fmt.Sprintf("%s %d\n", day, sum)))
						if de.Duration != 0 || !isWeekend {
							output = append(output, fmt.Sprintf("%d", sum))
						} else {
							output = append(output, " ")
						}
					}
					_, err = in.Write([]byte(strings.Join(output, "\t")))
				}
			}
		}
	case 3: // right click, start/stop
		if m.currentTimeEntry.ID != "" {
			_, err := m.clockifyClient.StopTimeEntry(m.currentTimeEntry)
			m.log.Infof("Stop time entry %+v", err)
			m.currentTimeEntry = clockify.TimeEntry{}
			m.currentName = 0
		} else {
			xlog.Info("Start time entry")
			if m.clockifyUser.DefaultWorkspace != "" {
				var newTimeEntry = clockify.TimeEntry{
					Description: m.tickets[0].name,
					WorkspaceID: m.clockifyUser.DefaultWorkspace,
					UserID:      m.clockifyUser.ID,
					TimeInterval: clockify.TimeInterval{
						Start: time.Now(),
					},
					ProjectID: m.tickets[0].PID,
				}
				m.currentTimeEntry, _ = m.clockifyClient.StartTimeEntry(newTimeEntry)
			}
		}
	case 4: // scroll up, increase
		m.currentName = m.currentName + 1
		if m.currentName >= len(m.tickets) {
			m.currentName = 0
		}
		m.currentTimeEntry.Description = m.tickets[m.currentName].name
		m.currentTimeEntry.ProjectID = m.tickets[m.currentName].PID
		m.updateTimer.SafeReset(time.Second * 1)
		m.updateTimeEntry = m.currentTimeEntry
	case 5: // scroll down, decrease
		m.currentName = m.currentName - 1
		if m.currentName < 0 {
			m.currentName = len(m.tickets) - 1
		}
		m.currentTimeEntry.Description = m.tickets[m.currentName].name
		m.currentTimeEntry.ProjectID = m.tickets[m.currentName].PID
		m.updateTimer.SafeReset(time.Second * 1)
		m.updateTimeEntry = m.currentTimeEntry
	}
	info = m.UpdateInfo(info)
	return &info, nil
}

func (m *Clockify) calcRemainingTime() {
	now := time.Now()
	t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	timeEntries, err := m.clockifyClient.GetTimeEntries(m.clockifyUser.DefaultWorkspace,
		m.clockifyUser.ID,
		t,
		time.Time{})
	m.log.Debugf("calcRemainingTime %+v", timeEntries, err)
	m.todayDuration = "00s"
	if err == nil {
		var dur float64
		for _, timeEntry := range timeEntries {
			dur += timeEntry.DurationInSec()
		}
		if int(dur) > 0 {
			m.todayDuration = prettyPrintDuration(int(dur), false)
		}
	}
}

func (m *Clockify) getCurrentTimeEntry() {
	var err error
	currentTimeEntry, err := m.clockifyClient.GetCurrentTimeEntry(m.clockifyUser.DefaultWorkspace, m.clockifyUser.ID)
	if err != nil {
		m.log.Error("getCurrentTimeEntry", err)
		return
	}
	if m.updateTimeEntry.ID != "" {
		return
	}
	m.currentTimeEntry = currentTimeEntry
	if len(m.currentTimeEntry.Description) > 50 {
		m.currentTimeEntry.Description = m.currentTimeEntry.Description[0:50] + "..."
	}
	if m.currentTimeEntry.ProjectID != "" {
		proj := m.projects.FindById(m.currentTimeEntry.ProjectID)
		if proj == nil {
			m.log.Error("Project not found", m.currentTimeEntry.ProjectID)
			return
		}
		m.currentTimeEntry.Description += fmt.Sprintf(" - %s", proj.Name)
	}
}

func (m *Clockify) updateCurrentTimeEntry() {
	m.Lock()
	defer m.Unlock()
	id := m.updateTimeEntry.ID
	if id == "" {
		return
	}
	m.log.Info("Update", m.updateTimeEntry)
	_, err := m.clockifyClient.UpdateTimeEntry(m.updateTimeEntry)
	if err != nil {
		return
	}
	if id == m.updateTimeEntry.ID {
		m.updateTimeEntry = clockify.TimeEntry{}
	} else {
		m.updateTimer.SafeReset(time.Second * 1)
	}
}

func (m *Clockify) updateProjectsAndTasks() {
	var err error
	m.projects, err = m.clockifyClient.GetWorkspaceProjects(m.clockifyUser.DefaultWorkspace)
	if err != nil {
		m.log.Error("Unable to get workspace projects", err)
	}

	var tickets []cticket
	for _, ticketName := range m.TicketNames {
		t := cticket{
			name: ticketName.Name,
		}
		proj := m.projects.FindByName(ticketName.Project)
		if proj == nil {
			m.log.Errorf("Project not found: %s", ticketName.Project)
			continue
		}
		t.PID = proj.ID
		tickets = append(tickets, t)
	}
	if len(tickets) > 0 {
		m.tickets = tickets
	}
}
