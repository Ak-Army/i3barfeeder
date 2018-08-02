package modules

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

const (
	secondsPerMinute = 60
	secondsPerHour   = 60 * 60
)

type Toggl struct {
	gobar.ModuleInterface
	toggl       toggl
	defaultWID  int64
	ticketNames []string
}

var currentTimeEntry TimeEntry
var updateTimeEntry TimeEntry
var todayDuration string
var currentName = 0
var updateTimer *time.Timer

func (module *Toggl) InitModule(config gobar.Config) error {
	if apiToken, ok := config["apiToken"].(string); ok {
		module.toggl = GetToggleClient(apiToken)
	} else {
		return errors.New("ApiToken not found")
	}
	if defaultWID, ok := config["defaultWID"].(float64); ok {
		module.defaultWID = int64(defaultWID)
	}

	if ticketNames, ok := config["ticketNames"].([]interface{}); ok {
		for _, item := range ticketNames {
			if itemString, ok := item.(string); ok {
				module.ticketNames = append(module.ticketNames, itemString)
			}
		}
	}
	module.calcRemainingTime()

	ticker := time.NewTicker(time.Second)
	go func() {
		for t := range ticker.C {
			switch {
			case t.Second()%10 == 0:
				if updateTimeEntry.ID == 0 {
					module.getCurrentTimeEntry()
				}
			case t.Minute() > 0 && t.Minute()%5 == 0:
				module.calcRemainingTime()
			}
		}
	}()

	updateTimer = time.AfterFunc(time.Second*3, func() {
		module.updateCurrentTimeEntry()
	})
	updateTimer.Stop()

	return nil
}

func (module Toggl) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if currentTimeEntry.ID != 0 {
		prettyTime := fmt.Sprintf("%s / %s", prettyPrintDuration(int(currentTimeEntry.GetDuration()), true), todayDuration)
		shortDesc := currentTimeEntry.Description
		if len(currentTimeEntry.Description) > 7 {
			shortDesc = currentTimeEntry.Description[0:7]
		}
		info.ShortText = fmt.Sprintf("%s - %s", shortDesc, prettyTime)
		info.FullText = fmt.Sprintf("%s - %s", currentTimeEntry.Description, prettyTime)
	} else {
		info.ShortText = fmt.Sprintf("%s", todayDuration)
		info.FullText = fmt.Sprintf("%s", info.ShortText)
	}
	return info
}

//{"name":"Toggl","instance":"id_0","button":5,"x":2991,"y":12}
func (module Toggl) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	currentTimeEntry, _ = module.toggl.GetCurrentTimeEntry()
	updateTimer.Stop()
	updateTimeEntry = TimeEntry{}
	switch cm.Button {
	case 2: //middle button
		now := time.Now()
		from := now.AddDate(0, -1, 0)
		entries, err := module.toggl.GetTimeEntries(from, now)
		daySums := make(map[string]int64)
		if err == nil {
			for _, entry := range entries {
				day := entry.Start[0:10]
				daySums[day] = daySums[day] + entry.Duration
			}

			copyCmd := exec.Command("xclip", "-selection", "c")
			in, err := copyCmd.StdinPipe()
			if err == nil {
				err = copyCmd.Start()
				defer copyCmd.Wait()
				defer in.Close()
				if err == nil {
					var output []string
					for day, daySum := range daySums {
						output = append(output, day+" "+strconv.FormatInt((daySum-28800)/60, 10))
					}
					sort.Strings(output)
					_, err = in.Write([]byte(strings.Join(output, "\n")))
				}
			}
		}
	case 3: //right click, start/stop
		if currentTimeEntry.ID != 0 {
			module.toggl.StopTimeEntry(currentTimeEntry)
			currentTimeEntry = TimeEntry{}
		} else {
			if module.defaultWID != 0 {
				var newTimeEntry = TimeEntry{
					Description: module.ticketNames[0],
					WID:         module.defaultWID,
					CreatedWith: "hunyi",
				}
				currentTimeEntry, _ = module.toggl.StartTimeEntry(newTimeEntry)
			}
		}
	case 4: //scroll up, increase
		currentName = currentName + 1
		if currentName >= len(module.ticketNames) {
			currentName = 0
		}
		currentTimeEntry.Description = module.ticketNames[currentName]
		updateTimeEntry = currentTimeEntry
		updateTimer.Reset(time.Second * 3)
	case 5: //scroll down, decrease
		currentName = currentName - 1
		if currentName < 0 {
			currentName = len(module.ticketNames) - 1
		}
		currentTimeEntry.Description = module.ticketNames[currentName]
		updateTimeEntry = currentTimeEntry
		updateTimer.Reset(time.Second * 3)
	}
	info = module.UpdateInfo(info)
	return &info, nil
}

func (module Toggl) calcRemainingTime() {
	now := time.Now()
	t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	timeEntries, err := module.toggl.GetTimeEntries(t, time.Time{})
	todayDuration = "0 / 0"
	if err == nil {
		var dur float64 = 0.0
		for _, timeEntry := range timeEntries {
			dur += timeEntry.GetDuration()
		}
		if int(dur) > 0 {
			todayDuration = prettyPrintDuration(int(dur), false)
		}
	}
}

func (module Toggl) getCurrentTimeEntry() {
	//var err error
	currentTimeEntry, _ = module.toggl.GetCurrentTimeEntry()
	//fmt.Printf("%+v\n",err);
}

func (module Toggl) updateCurrentTimeEntry() {
	module.toggl.UpdateTimeEntry(updateTimeEntry)
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

type TimeEntry struct {
	ID          int64    `json:"id,omitempty"`
	Description string   `json:"description"`
	WID         int64    `json:"wid,omitempty"`
	PID         int64    `json:"pid,omitempty"`
	TID         int64    `json:"tid,omitempty"`
	Billable    bool     `json:"billable,omitempty"`
	Start       string   `json:"start,omitempty"`
	Stop        string   `json:"stop,omitempty"`
	Duration    int64    `json:"duration,omitempty"`
	CreatedWith string   `json:"created_with"`
	Tags        []string `json:"tags,omitempty"`
	Duronly     bool     `json:"duronly,omitempty"`
	At          string   `json:"at,omitempty"`
}

type toggl struct {
	client    *http.Client
	transport *http.Transport
	baseUrl   string
	apiToken  string
}

type currentResponse struct {
	Data TimeEntry `json:"data"`
}

type createTimeEntry struct {
	TimeEntry TimeEntry `json:"time_entry"`
}

const date_ISO8601 = "2006-01-02T15:04:05+00:00"

func GetToggleClient(apiToken string) toggl {
	transport := &http.Transport{}
	baseUrl := "https://toggl.com/api/v8"
	client := &http.Client{Transport: transport}

	return toggl{
		client:    client,
		transport: transport,
		baseUrl:   baseUrl,
		apiToken:  apiToken,
	}
}

func (toggle toggl) request(method string, endpoint string, param interface{}) (response []byte, err error) {
	// format param
	var bodyText []byte
	if param != nil {
		bodyText, err = json.Marshal(param)
		if err != nil {
			return
		}
	}

	req, err := http.NewRequest(method, toggle.baseUrl+endpoint, bytes.NewReader(bodyText))
	if err != nil {
		return
	}
	// format token
	basic := base64.StdEncoding.EncodeToString([]byte(toggle.apiToken + ":api_token"))
	req.Header.Set("Authorization", "Basic "+basic)

	res, err := toggle.client.Do(req)

	if err != nil {
		return
	}
	defer res.Body.Close()
	contentType := res.Header.Get("content-type")
	if contentType == "application/json; charset=utf-8" {
		response, err = ioutil.ReadAll(res.Body)
	} else if !(res.StatusCode >= 200 && res.StatusCode < 300) {
		err = errors.New("Response wrong status code")
	}

	return
}

func (toggle toggl) GetCurrentTimeEntry() (TimeEntry, error) {
	res, err := toggle.request("GET", "/time_entries/current", nil)
	if err != nil {
		return TimeEntry{}, err
	}
	var response = &currentResponse{}
	err = json.Unmarshal(res, response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response.Data, nil
}

func (toggle toggl) StartTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	createTimeEntry := createTimeEntry{TimeEntry: timeEntry}
	res, err := toggle.request("POST", "/time_entries/start", createTimeEntry)
	if err != nil {
		return TimeEntry{}, err
	}
	var response = &currentResponse{}
	err = json.Unmarshal(res, response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response.Data, nil
}

func (toggle toggl) StopTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	idString := strconv.FormatInt(timeEntry.ID, 10)
	res, err := toggle.request("GET", "/time_entries/"+idString+"/stop", nil)

	if err != nil {
		return TimeEntry{}, err
	}
	var response = &currentResponse{}
	err = json.Unmarshal(res, response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response.Data, nil
}

func (toggle toggl) GetTimeEntry(id int) (TimeEntry, error) {
	idString := strconv.Itoa(id)
	res, err := toggle.request("GET", "/time_entries/"+idString, nil)
	if err != nil {
		return TimeEntry{}, err
	}
	var response = &currentResponse{}
	err = json.Unmarshal(res, response)
	if err != nil {
		return TimeEntry{}, err
	}
	return response.Data, nil
}

func (toggle toggl) GetTimeEntries(fromDate time.Time, toDate time.Time) ([]TimeEntry, error) {
	var response = []TimeEntry{}
	endpoint := "/time_entries"
	if !fromDate.IsZero() {
		endpoint += "?start_date=" + url.QueryEscape(fromDate.Format(date_ISO8601))
		if !toDate.IsZero() {
			endpoint += "&end_date=" + url.QueryEscape(toDate.Format(date_ISO8601))
		}
	}
	res, err := toggle.request("GET", endpoint, nil)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(res, &response)
	if err != nil {
		return response, err
	}
	return response, nil
}

func (toggle toggl) UpdateTimeEntry(timeEntry TimeEntry) (TimeEntry, error) {
	idString := strconv.FormatInt(timeEntry.ID, 10)
	createTimeEntry := createTimeEntry{TimeEntry: timeEntry}
	res, err := toggle.request("PUT", "/time_entries/"+idString, createTimeEntry)

	if err != nil {
		return TimeEntry{}, err
	}
	var response = &currentResponse{}
	err = json.Unmarshal(res, response)
	if err != nil {
		return TimeEntry{}, err
	}
	updateTimeEntry = TimeEntry{}
	return response.Data, nil
}

func (timeEntry TimeEntry) GetDuration() float64 {
	var timeDur time.Duration
	var err error
	if timeEntry.Duration < 0 {
		timeStart, err := time.Parse(date_ISO8601, timeEntry.Start)
		if err != nil {
			return 0.0
		}
		timeDur = time.Since(timeStart)
	} else {
		durString := fmt.Sprintf("%ds", timeEntry.Duration)
		timeDur, err = time.ParseDuration(durString)
		if err != nil {
			return 0.0
		}
	}
	return timeDur.Seconds()
}
