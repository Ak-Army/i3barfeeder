package modules

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ak-Army/xlog"

	"github.com/Ak-Army/i3barfeeder/gobar"
	"github.com/Ak-Army/i3barfeeder/internal/toggl"
)

const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * 60
)

func init() {
	gobar.AddModule("Toggl", func() gobar.ModuleInterface {
		return &Toggl{todayDuration: "00s"}
	})
}

type Toggl struct {
	sync.Mutex
	gobar.ModuleInterface
	DefaultWID      int64        `json:"defaultWID"`
	ApiToken        string       `json:"apiToken"`
	TicketNames     []ticketName `json:"ticketNames"`
	tickets         []ticket
	currentTimeEntry toggl.TimeEntry
	updateTimeEntry toggl.TimeEntry
	todayDuration   string
	currentName     int
	updateTimer     *time.Timer
	log             xlog.Logger
	projects        toggl.Projects
	togglClient     toggl.Client
}

type ticketName struct {
	Name     string     `json:"name"`
	Projects [][]string `json:"projects"`
	YTId     string     `json:"ytId"`
}
type ticket struct {
	name string
	PID int64
	TID int64
}
type dayEntry struct {
	Duration int64
	Date     time.Time
}

func (m *Toggl) InitModule(config json.RawMessage, log xlog.Logger) error {
	m.log = log
	if err := json.Unmarshal(config, m); err != nil {
		return err
	}
	m.togglClient = toggl.NewClient(m.ApiToken)
	m.calcRemainingTime()
	m.updateProjectsAndTasks()

	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for t := range ticker.C {
			if m.updateTimeEntry.ID == 0 {
				m.getCurrentTimeEntry()
			}
			if t.Minute() > 0 && t.Minute()%5 == 0 {
				m.calcRemainingTime()
				m.updateProjectsAndTasks()
			}
		}
	}()
	m.updateTimer = time.AfterFunc(time.Second, func() {
		m.updateCurrentTimeEntry()
	})
	return nil
}

func (m *Toggl) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if m.currentTimeEntry.ID != 0 {
		prettyTime := fmt.Sprintf("%s / %s", prettyPrintDuration(int(m.currentTimeEntry.DurationInSec()), true), m.todayDuration)
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
func (m *Toggl) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	m.Lock()
	defer m.Unlock()
	m.currentTimeEntry, _ = m.togglClient.GetCurrentTimeEntry()
	m.updateTimer.Stop()
	m.updateTimeEntry = toggl.TimeEntry{}
	switch cm.Button {
	case 2: // middle button
		now := time.Now()
		from := now.AddDate(0, -1, 0)
		entries, err := m.togglClient.GetTimeEntries(from, now)
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
				if timeEntry.Duration < 0 {
					continue
				}
				day := timeEntry.Start.Format("2006-01-02")
				days[day] = dayEntry{
					Duration: days[day].Duration + timeEntry.Duration,
					Date:     timeEntry.Start,
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
		if m.currentTimeEntry.ID != 0 {
			m.togglClient.StopTimeEntry(m.currentTimeEntry)
			m.currentTimeEntry = toggl.TimeEntry{}
		} else {
			if m.DefaultWID != 0 {
				var newTimeEntry = toggl.TimeEntry{
					Description: m.tickets[0].name,
					PID: m.tickets[0].PID,
					TID: m.tickets[0].TID,
					WID:         m.DefaultWID,
					CreatedWith: "hunyi",
				}
				m.currentTimeEntry, _ = m.togglClient.StartTimeEntry(newTimeEntry)
			}
		}
	case 4: // scroll up, increase
		m.currentName = m.currentName + 1
		if m.currentName >= len(m.tickets) {
			m.currentName = 0
		}
		m.currentTimeEntry.Description = m.tickets[m.currentName].name
		m.currentTimeEntry.PID = m.tickets[m.currentName].PID
		m.currentTimeEntry.TID = m.tickets[m.currentName].TID
		m.updateTimeEntry = m.currentTimeEntry
		m.updateTimer.Reset(time.Second * 1)
	case 5: // scroll down, decrease
		m.currentName = m.currentName - 1
		if m.currentName < 0 {
			m.currentName = len(m.tickets) - 1
		}
		m.currentTimeEntry.Description = m.tickets[m.currentName].name
		m.currentTimeEntry.PID = m.tickets[m.currentName].PID
		m.currentTimeEntry.TID = m.tickets[m.currentName].TID
		m.updateTimeEntry = m.currentTimeEntry
		m.updateTimer.Reset(time.Second * 1)
	}
	info = m.UpdateInfo(info)
	return &info, nil
}

func (m *Toggl) calcRemainingTime() {
	now := time.Now()
	t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	timeEntries, err := m.togglClient.GetTimeEntries(t, time.Time{})
	m.log.Debugf("calcRemainingTime %+v", timeEntries)
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

func (m *Toggl) getCurrentTimeEntry() {
	var err error
	if m.currentTimeEntry, err = m.togglClient.GetCurrentTimeEntry(); err != nil {
		m.log.Error("getCurrentTimeEntry", err)
		return
	}
	if len(m.currentTimeEntry.Description) > 50 {
		m.currentTimeEntry.Description = m.currentTimeEntry.Description[0:50]+"..."
	}
	proj := m.projects.FindById(m.currentTimeEntry.PID)
	if proj == nil {
		return
	}
	task := proj.Tasks.FindById(m.currentTimeEntry.TID)
	if task == nil {
		return
	}
	m.currentTimeEntry.Description+= fmt.Sprintf(" - %s / %s", proj.Name, task.Name)

}

func (m *Toggl) updateCurrentTimeEntry() {
	m.log.Info("Update", m.updateTimeEntry)
	m.togglClient.UpdateTimeEntry(m.updateTimeEntry)
	m.updateTimeEntry = toggl.TimeEntry{}
}

func (m *Toggl) updateProjectsAndTasks() {
	var err error
	m.projects, err = m.togglClient.GetWorkspaceProjects(m.DefaultWID)
	if err !=  nil {
		m.log.Error("Unable to get workspace projects")
		return
	}
	for _, p := range m.projects {
		p.Tasks, err = m.togglClient.GetProjectTasks(p.ID)
		if err !=  nil {
			m.log.Errorf("Unable to get project tasks: %d %s", p.ID, p.Name)
		}
	}
	var tickets []ticket
	for _, t := range m.TicketNames {
		for _, p := range t.Projects {
			proj := m.projects.FindByName(p[0])
			if proj == nil {
				m.log.Errorf("Project not found: %s (%s)", p[0], t.Name)
				continue
			}
			task := proj.Tasks.FindByName(p[1])
			if task == nil {
				m.log.Errorf("Task not found in %s project: %s (%s)", proj.Name, p[1], t.Name)
				continue
			}
			tickets = append(tickets, ticket{
				name: fmt.Sprintf("%s %s %s - %s / %s ", t.YTId, p[2], t.Name, proj.Name, task.Name),
				PID: proj.ID,
				TID: task.ID,
			})
		}
	}
	if len(tickets) > 0 {
		m.Lock()
		m.tickets = tickets
		m.Unlock()
	}
}

func prettyPrintDuration(sec int, withSec bool) string {
	var hour, min int
	hour = sec / secondsPerHour
	sec -= hour * secondsPerHour
	min = sec / secondsPerMinute
	sec -= min * secondsPerMinute

	returnString := ""
	if hour > 0 {
		returnString = fmt.Sprintf("%s%02dH ", returnString, hour)
	}
	if min > 0 {
		returnString = fmt.Sprintf("%s%02dm ", returnString, min)
	}
	if sec > 0 && withSec {
		returnString = fmt.Sprintf("%s%02ds", returnString, sec)
	}

	return returnString
}
