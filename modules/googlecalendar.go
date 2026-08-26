package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Ak-Army/config"
	"github.com/Ak-Army/xlog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/Ak-Army/i3barfeeder/gobar"
)

func init() {
	gobar.AddModule("GCal", func() gobar.ModuleInterface {
		return &GCal{
			SecretFile: "credentials.json",
			TokenFile:  "token.json",
		}
	})
}

type event struct {
	*calendar.Event

	meetingLink string
	clicked     bool
}

type GCal struct {
	gobar.BaseModule
	SecretFile  string `config:"secretFile"`
	TokenFile   string `config:"tokenFile"`
	Email       string `config:"email"`
	MeetingLink map[string]*struct {
		Regex  string `config:"regex"`
		Simple string `config:"simple"`
		regex  *regexp.Regexp
	} `config:"meetingLink"`
	log           xlog.Logger
	googleService *calendar.Service
	info          string

	// UpdateInfo runs on the block goroutine while HandleClick runs on the bar's
	// single click goroutine, and both reach this same instance: createBar copies
	// the Block struct, not the module pointer. Everything below, including the
	// clicked flag of the events in the slice, is only safe under mu.
	mu           sync.Mutex
	lastQuery    time.Time
	events       []*event
	currentEvent *event
	leftClick    time.Time
}

func (m *GCal) InitModule(c *config.SubConfig, log xlog.Logger) error {
	m.log = log
	if c != nil {
		if err := c.Load(m); err != nil {
			return err
		}
	}
	for s, l := range m.MeetingLink {
		if l.Regex != "" {
			r, err := regexp.Compile(l.Regex)
			if err != nil {
				delete(m.MeetingLink, s)
				m.log.Warnf("Wrong regex for link: %s", s, err)
				continue
			}
			m.MeetingLink[s].regex = r
		}
	}
	ctx := context.Background()
	b, err := os.ReadFile(m.SecretFile)
	if err != nil {
		return err
	}
	// If modifying these scopes, delete your previously saved token.json.
	gc, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return err
	}
	client := m.getClient(gc)
	if client == nil {
		return nil
	}

	m.googleService, err = calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		m.info = err.Error()
		return nil
	}
	return nil
}

func (m *GCal) UpdateInfo(info gobar.BlockInfo) gobar.BlockInfo {
	if m.info != "" {
		info.TextColor = "#FFFFFF"
		info.ShortText = m.info
		info.FullText = m.info
		m.mu.Lock()
		m.currentEvent = nil
		m.mu.Unlock()
		return info
	}
	if m.reloadDue() {
		m.reloadEvents()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentEvent == nil {
		info.ShortText = "No events"
		info.FullText = "No upcoming events found."
		if event := m.getCurrentEvent(); event != nil {
			m.showEvent(event, &info)
		}
	} else {
		m.showEvent(m.currentEvent, &info)
	}
	return info
}

func (m *GCal) reloadDue() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if time.Since(m.lastQuery) <= time.Hour/2 {
		return false
	}
	m.lastQuery = time.Now()
	return true
}

func (m *GCal) reloadEvents() {
	m.log.Info("Load google events")
	m.mu.Lock()
	t := m.lastQuery.Truncate(time.Hour * 24)
	m.mu.Unlock()

	gevents, err := m.googleService.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t.Format(time.RFC3339)).MaxResults(10).
		OrderBy("startTime").Do()
	if err != nil {
		m.log.Errorf("Unable to retrieve next ten of the user's events: %v", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	var evs []*event
	for _, e := range gevents.Items {
		ev := &event{Event: e}
		for _, oe := range m.events {
			if oe.Id == e.Id {
				ev.clicked = oe.clicked
			}
		}
		ev.meetingLink = m.findMeetingLink(ev)
		evs = append(evs, ev)
	}
	m.events = evs
}

func (m *GCal) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	switch cm.Button {
	case 1: // left click, a double click within a second reloads the events
		m.mu.Lock()
		reload := !m.leftClick.IsZero() && time.Since(m.leftClick) <= time.Second
		if reload {
			m.leftClick = time.Time{}
		} else {
			m.leftClick = time.Now()
		}
		m.mu.Unlock()
		if reload {
			m.reloadEvents()
		}
		return &info, nil
	case 2: // middle button
		m.mu.Lock()
		defer m.mu.Unlock()
		m.currentEvent = nil
		if e := m.getCurrentEvent(); e != nil {
			m.showEvent(e, &info)
		}

		return &info, nil
	case 3: // right click, join zoom
		m.mu.Lock()
		defer m.mu.Unlock()
		e := m.currentEvent
		if e == nil {
			e = m.getCurrentEvent()
		}
		if e == nil {
			return nil, nil
		}
		meetingLink := m.findMeetingLink(e)
		e.clicked = true
		if meetingLink != "" {
			m.openURL(meetingLink)
		} else {
			s, _ := json.Marshal(e)
			m.log.Warnf("unable to find zoom link: %s", string(s))
			m.log.Warnf("unable to find zoom link: %s", e.Description)
		}
	case 4: // scroll up, decrease
		m.mu.Lock()
		defer m.mu.Unlock()
		e := m.currentEvent
		if e == nil {
			e = m.getCurrentEvent()
		}
		if e == nil {
			return nil, nil
		}
		l := len(m.events) - 1
		for i, item := range m.events {
			if item.Id == e.Id && i < l {
				m.showEvent(m.events[i+1], &info)
				m.currentEvent = m.events[i+1]
				return &info, nil
			}
		}
	case 5: // scroll down, decrease
		m.mu.Lock()
		defer m.mu.Unlock()
		e := m.currentEvent
		if e == nil {
			e = m.getCurrentEvent()
		}
		if e == nil {
			return nil, nil
		}
		for i, item := range m.events {
			if item.Id == e.Id && i > 0 {
				m.showEvent(m.events[i-1], &info)
				m.currentEvent = m.events[i-1]
				return &info, nil
			}
		}
	}
	return nil, nil
}

func (m *GCal) findMeetingLink(event *event) string {
	if event == nil {
		return ""
	}
	if event.meetingLink != "" {
		return event.meetingLink
	}
	for s, l := range m.MeetingLink {
		if event.ConferenceData != nil &&
			len(event.ConferenceData.EntryPoints) > 0 {
			for _, e := range event.ConferenceData.EntryPoints {
				if l.regex != nil {
					url := l.regex.FindString(e.Uri)
					if url != "" {
						return url
					}
				}
				if strings.Contains(e.Uri, s) {
					return e.Uri
				}
			}
		}
		if l.regex != nil {
			url := l.regex.FindString(event.Location)
			if url != "" {
				return url
			}
		}
		if event.Location == l.Simple && l.Simple != "" {
			return event.Location
		}
		if strings.Contains(event.Description, l.Simple) {
			if l.regex != nil {
				url := l.regex.FindString(event.Description)
				if url != "" {
					return url
				}
			} else {
				return l.Simple
			}
		}
		lines := strings.Split(event.Description, "\n")
		linesLen := len(lines)
		for i, line := range lines {
			if line == l.Simple {
				if l.regex != nil && i+1 < linesLen {
					url := l.regex.FindString(lines[i+1])
					if url != "" {
						return url
					}
				} else {
					return l.Simple
				}
			}
		}
	}
	return ""
}

// getCurrentEvent must be called with m.mu held.
func (m *GCal) getCurrentEvent() *event {
	t := time.Now().Add(10 * time.Minute)
	var maybeFound *event
	for _, item := range m.events {
		if !hasTimes(item) || m.isAllDayEvent(item) {
			continue
		}
		endDateTime, err := time.Parse(time.RFC3339, item.End.DateTime)
		if err != nil {
			continue
		}
		if t.Before(endDateTime) {
			if m.isDeclined(item) {
				maybeFound = item
				continue
			}
			if !m.isAccepted(item) {
				maybeFound = item
				continue
			}
			return item
		} else if maybeFound != nil {
			return maybeFound
		}
	}
	return nil
}

// showEvent must be called with m.mu held; it writes event.clicked.
func (m *GCal) showEvent(event *event, info *gobar.BlockInfo) {
	if !hasTimes(event) {
		return
	}
	startDateTime, err := time.Parse(time.RFC3339, event.Start.DateTime)
	if err != nil {
		return
	}
	endDateTime, err := time.Parse(time.RFC3339, event.End.DateTime)
	if err != nil {
		return
	}
	info.TextColor = "#FFFFFF"
	t := time.Now()
	if t.After(startDateTime.Add(-10 * time.Minute)) {
		info.TextColor = "#c92822"
	}
	if t.After(startDateTime) {
		info.TextColor = "#30b856"
	}
	if m.isAccepted(event) && !event.clicked && event.meetingLink != "" {
		sub := t.Sub(startDateTime)
		if sub > -1*time.Minute && sub < time.Minute {
			event.clicked = true
			m.openURL(event.meetingLink)
		}
	}

	info.ShortText = fmt.Sprintf("%s (%s)", event.Summary, startDateTime.Format("15:04"))
	info.FullText = fmt.Sprintf("%s (%s-%s)", event.Summary, startDateTime.Format("15:04"), endDateTime.Format("15:04"))
	if m.isDeclined(event) {
		info.ShortText += " [D]"
		info.FullText += " [DECLINED]"
	}
	return
}

func (m *GCal) isDeclined(event *event) bool {
	for _, a := range event.Attendees {
		if a.Email == m.Email {
			if a.ResponseStatus == "declined" {
				return true
			}
		}
	}
	return false
}

func (m *GCal) isAccepted(event *event) bool {
	for _, a := range event.Attendees {
		if a.Email == m.Email {
			if a.ResponseStatus == "accepted" {
				return true
			}
		}
	}
	return false
}

func (m *GCal) isAllDayEvent(event *event) bool {
	return hasTimes(event) && (event.Start.Date != "" || event.End.Date != "")
}

func hasTimes(e *event) bool {
	return e != nil && e.Start != nil && e.End != nil
}

func (m *GCal) getClient(config *oauth2.Config) *http.Client {
	tok, err := m.tokenFromFile()
	if err != nil {
		tok = m.getTokenFromWeb(config)
		if tok == nil {
			m.log.Error("No token obtained, not caching one")
			return nil
		}
		m.saveToken(tok)
	}
	return config.Client(context.Background(), tok)
}

const authTimeout = 2 * time.Minute

func (m *GCal) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	ch := make(chan string, 1)
	randState := fmt.Sprintf("st%d", time.Now().UnixNano())
	ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/favicon.ico" {
			http.Error(rw, "", 404)
			return
		}
		if req.FormValue("state") != randState {
			m.log.Infof("State doesn't match: req = %#v", req)
			http.Error(rw, "", 500)
			return
		}
		if code := req.FormValue("code"); code != "" {
			fmt.Fprintf(rw, "<h1>Success</h1>Authorized.")
			rw.(http.Flusher).Flush()
			ch <- code
			return
		}
		m.log.Infof("no code")
		http.Error(rw, "", 500)
	}))
	defer ts.Close()

	config.RedirectURL = ts.URL
	authURL := config.AuthCodeURL(randState)
	go m.openURL(authURL)
	m.log.Info("Authorize this app at: %s", authURL)
	var code string
	select {
	case code = <-ch:
	case <-time.After(authTimeout):
		m.log.Errorf("No authorization received within %s", authTimeout)
		return nil
	}
	m.log.Infof("Got code: %s", code)

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		m.log.Errorf("Token exchange error: %v", err)
		return nil
	}
	return token
}

func (m *GCal) openURL(url string) {
	if err := openURL(url); err != nil {
		m.log.Infof("Error opening URL in browser: %s", err)
	}
}

func (m *GCal) tokenFromFile() (*oauth2.Token, error) {
	f, err := os.Open(m.TokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	if err := json.NewDecoder(f).Decode(tok); err != nil {
		return nil, err
	}
	if tok.AccessToken == "" && tok.RefreshToken == "" {
		return nil, fmt.Errorf("%s holds no usable token", m.TokenFile)
	}
	return tok, nil
}

// Saves a token to a file path.
func (m *GCal) saveToken(token *oauth2.Token) {
	//fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(m.TokenFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		m.log.Errorf("Unable to cache oauth token: %v", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}
