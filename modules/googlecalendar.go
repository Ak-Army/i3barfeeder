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
	"google.golang.org/api/option"

	"github.com/Ak-Army/i3barfeeder/gobar"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

var zoomRegex *regexp.Regexp

func init() {
	gobar.AddModule("GCal", func() gobar.ModuleInterface {
		return &GCal{
			SecretFile: "credentials.json",
			TokenFile:  "token.json",
		}
	})
	zoomRegex = regexp.MustCompile(`https:\/\/([^.]+.)?zoom\.us\/[^\\" \n]+`)
}

type GCal struct {
	gobar.ModuleInterface
	SecretFile    string `json:"secretFile"`
	TokenFile     string `json:"tokenFile"`
	Email         string `json:"email"`
	log           xlog.Logger
	googleService *calendar.Service
	lastQuery     time.Time
	events        *calendar.Events
	info          string
	currentEvent  *calendar.Event
	eventLock     sync.Mutex
}

func (m *GCal) InitModule(config json.RawMessage, log xlog.Logger) error {
	m.log = log
	if config != nil {
		if err := json.Unmarshal(config, m); err != nil {
			return err
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
		events, err := m.googleService.Events.List("primary").ShowDeleted(false).
			SingleEvents(true).TimeMin(t.Format(time.RFC3339)).MaxResults(10).
			OrderBy("startTime").Do()
		if err != nil {
			m.log.Errorf("Unable to retrieve next ten of the user's events: %v", err)
		} else {
			m.events = events
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
	switch cm.Button {
	case 2: // middle button
		m.eventLock.Lock()
		m.currentEvent = nil
		m.eventLock.Unlock()
		event := m.getCurrentEvent()
		m.showEvent(event, &info)

		return &info, nil
	case 3: // right click, join zoom
		m.eventLock.Lock()
		event := m.currentEvent
		m.eventLock.Unlock()
		if event == nil {
			event = m.getCurrentEvent()
		}
		zoomLink := m.findVideoLink(event)
		if zoomLink != "" {
			m.openURL(zoomLink)
		} else {
			s, _ := json.Marshal(event)
			m.log.Warnf("unable to find zoom link: %s", string(s))
			m.log.Warnf("unable to find zoom link: %s", event.Description)
		}
	case 4: // scroll up, decrease
		m.eventLock.Lock()
		event := m.currentEvent
		m.eventLock.Unlock()
		if event == nil {
			event = m.getCurrentEvent()
		}
		l := len(m.events.Items) - 1
		for i, item := range m.events.Items {
			if item.Id == event.Id && i < l {
				m.showEvent(m.events.Items[i+1], &info)
				m.eventLock.Lock()
				m.currentEvent = m.events.Items[i+1]
				m.eventLock.Unlock()
				return &info, nil
			}
		}
	case 5: // scroll down, decrease
		m.eventLock.Lock()
		event := m.currentEvent
		m.eventLock.Unlock()
		for i, item := range m.events.Items {
			if item.Id == event.Id && i > 0 {
				m.showEvent(m.events.Items[i-1], &info)
				m.eventLock.Lock()
				m.currentEvent = m.events.Items[i-1]
				m.eventLock.Unlock()
				return &info, nil
			}
		}
	}
	return nil, nil
}

func (m *GCal) findVideoLink(event *calendar.Event) string {
	if event.ConferenceData != nil &&
		len(event.ConferenceData.EntryPoints) > 0 {
		for _, e := range event.ConferenceData.EntryPoints {
			if strings.Contains(e.Uri, "https://zoom.us") ||
				strings.Contains(e.Uri, "https://meet.google.com") {
				return e.Uri
			}
		}
	}
	if strings.Contains(event.Location, "zoom.us/") {
		url := zoomRegex.FindString(event.Location)
		if url != "" {
			return url
		}
	}
	if event.Location == "https://rebrand.ly/sl3_zoom" {
		return event.Location
	}
	if strings.Contains(event.Description, "https://rebrand.ly/sl3_zoom") {
		return "https://rebrand.ly/sl3_zoom"
	}
	if strings.Contains(event.Description, "Join Zoom Meeting") {
		url := zoomRegex.FindString(event.Description)
		if url != "" {
			return url
		}
	}
	lines := strings.Split(event.Description, "\n")
	for i, line := range lines {
		if line == "Join Zoom Meeting" {
			url := zoomRegex.FindString(lines[i+1])
			if url != "" {
				return url
			}
		}
	}
	return ""
}

func (m *GCal) getCurrentEvent() *calendar.Event {
	t := time.Now().Add(10 * time.Minute)
	var maybeFound *calendar.Event
	for _, item := range m.events.Items {
		endDateTime, err := time.Parse(time.RFC3339, item.End.DateTime)
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
	for _, item := range m.events.Items {
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

func (m *GCal) showEvent(event *calendar.Event, info *gobar.BlockInfo) {
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
	info.ShortText = fmt.Sprintf("%s (%s)", event.Summary, startDateTime.Format("15:04"))
	info.FullText = fmt.Sprintf("%s (%s-%s)", event.Summary, startDateTime.Format("15:04"), endDateTime.Format("15:04"))
	if m.isDeclined(event) {
		info.ShortText += " [D]"
		info.FullText += " [DECLINED]"
	}
	return
}

func (m *GCal) isDeclined(event *calendar.Event) bool {
	for _, a := range event.Attendees {
		if a.Email == m.Email {
			if a.ResponseStatus == "declined" {
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
	try := []string{"xdg-open", "firefox", "open"}
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
