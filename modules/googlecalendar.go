package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

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
	gobar.ModuleInterface
	SecretFile  string `json:"secretFile"`
	TokenFile   string `json:"tokenFile"`
	Email       string `json:"email"`
	MeetingLink map[string]*struct {
		Regex  string `json:"regex"`
		Simple string `json:"simple"`
		regex  *regexp.Regexp
	} `json:"meetingLink"`
	log           xlog.Logger
	googleService *calendar.Service
	lastQuery     time.Time
	events        []*event
	info          string
	currentEvent  *event
	eventLock     sync.Mutex
}

func (m *GCal) InitModule(config json.RawMessage, log xlog.Logger) error {
	m.log = log
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
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
	b, err := ioutil.ReadFile(m.SecretFile)
	if err != nil {
		return err
	}
	// If modifying these scopes, delete your previously saved token.json.
	c, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		return err
	}
	client := m.getClient(c)
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
		m.currentEvent = nil
		return info
	}
	if time.Since(m.lastQuery) > time.Hour/2 {
		m.lastQuery = time.Now()
		t := m.lastQuery.Truncate(time.Hour * 24)
		gevents, err := m.googleService.Events.List("primary").ShowDeleted(false).
			SingleEvents(true).TimeMin(t.Format(time.RFC3339)).MaxResults(10).
			OrderBy("startTime").Do()
		if err != nil {
			m.log.Errorf("Unable to retrieve next ten of the user's events: %v", err)
		} else {
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
	}
	if m.currentEvent == nil {
		info.ShortText = "No events"
		info.FullText = "No upcoming events found."
		event := m.getCurrentEvent()
		m.showEvent(event, &info)
	} else {
		m.showEvent(m.currentEvent, &info)
	}
	return info
}

func (m *GCal) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	defer func() {

	}()
	switch cm.Button {
	case 2: // middle button
		m.eventLock.Lock()
		m.currentEvent = nil
		m.eventLock.Unlock()
		e := m.getCurrentEvent()
		m.showEvent(e, &info)

		return &info, nil
	case 3: // right click, join zoom
		m.eventLock.Lock()
		e := m.currentEvent
		m.eventLock.Unlock()
		if e == nil {
			e = m.getCurrentEvent()
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
		m.eventLock.Lock()
		e := m.currentEvent
		m.eventLock.Unlock()
		if e == nil {
			e = m.getCurrentEvent()
		}
		l := len(m.events) - 1
		for i, item := range m.events {
			if item.Id == e.Id && i < l {
				m.showEvent(m.events[i+1], &info)
				m.eventLock.Lock()
				m.currentEvent = m.events[i+1]
				m.eventLock.Unlock()
				return &info, nil
			}
		}
	case 5: // scroll down, decrease
		m.eventLock.Lock()
		e := m.currentEvent
		m.eventLock.Unlock()
		for i, item := range m.events {
			if item.Id == e.Id && i > 0 {
				m.showEvent(m.events[i-1], &info)
				m.eventLock.Lock()
				//m.currentEvent = m.events.Items[i-1]
				m.eventLock.Unlock()
				return &info, nil
			}
		}
	}
	return nil, nil
}

func (m *GCal) findMeetingLink(event *event) string {
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
				if l.regex != nil && linesLen < i {
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

func (m *GCal) getCurrentEvent() *event {
	t := time.Now().Add(10 * time.Minute)
	var maybeFound *event
	for _, item := range m.events {
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
	for _, item := range m.events {
		endDateTime, err := time.Parse(time.RFC3339, item.End.Date)
		if err != nil {
			continue
		}
		if t.Before(endDateTime) {
			if m.isDeclined(item) {
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

func (m *GCal) showEvent(event *event, info *gobar.BlockInfo) {
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

// Retrieve a token, saves the token, then returns the generated client.
func (m *GCal) getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tok, err := m.tokenFromFile()
	if err != nil {
		tok = m.getTokenFromWeb(config)
		m.saveToken(tok)
	}
	return config.Client(context.Background(), tok)
}

func (m *GCal) getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	ch := make(chan string)
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
	code := <-ch
	m.log.Infof("Got code: %s", code)

	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		m.log.Errorf("Token exchange error: %v", err)
		return nil
	}
	return token
}

func (m *GCal) openURL(url string) {
	try := []string{"xdg-open", "brave-browser", "google-chrome", "firefox", "open"}
	for _, bin := range try {
		err := exec.Command(bin, url).Run()
		if err == nil {
			return
		}
	}
	m.log.Infof("Error opening URL in browser.")
}

func (m *GCal) tokenFromFile() (*oauth2.Token, error) {
	f, err := os.Open(m.TokenFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
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
