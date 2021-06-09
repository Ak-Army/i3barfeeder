package modules

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Ak-Army/i3barfeeder/gobar"
	"github.com/Ak-Army/xlog"
	"google.golang.org/api/option"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

func init() {
	gobar.AddModule("GCal", func() gobar.ModuleInterface {
		return &GCal{
			SecretFile: "credentials.json",
			TokenFile:  "token.json",
		}
	})
}

type GCal struct {
	gobar.ModuleInterface
	SecretFile    string `json:"secretFile"`
	TokenFile     string `json:"tokenFile"`
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
		m.showCurrentEvent(&info)
	} else {
		m.showEvent(m.currentEvent, &info)
	}
	return info
}

func (m *GCal) HandleClick(cm gobar.ClickMessage, info gobar.BlockInfo) (*gobar.BlockInfo, error) {
	switch cm.Button {
	case 2: // middle button
		m.showCurrentEvent(&info)
		return &info, nil
	case 3: // right click, join zoom
		m.eventLock.Lock()
		event := m.currentEvent
		m.eventLock.Unlock()
		if event.ConferenceData != nil &&
			len(event.ConferenceData.EntryPoints) > 0 {
			for _, e := range event.ConferenceData.EntryPoints {
				if strings.Contains(e.Uri, "zoom.us/") {
					m.openURL(event.Location)
					return &info, nil
				}
			}
		}
		if strings.Contains(event.Location, "zoom.us/") {
			m.openURL(event.Location)
			return &info, nil
		}
		if event.Location == "https://rebrand.ly/sl3_zoom" {
			m.openURL(event.Location)
			return &info, nil
		}
		lines := strings.Split(event.Description, "\n")
		for i, line := range lines {
			if line == "Join Zoom Meeting" {
				m.openURL(lines[i+1])
				return &info, nil
			}
		}
	case 4: // scroll up, decrease
		m.eventLock.Lock()
		event := m.currentEvent
		m.eventLock.Unlock()
		l := len(m.events.Items) - 1
		for i, item := range m.events.Items {
			if item.Id == event.Id && i < l {
				m.showEvent(m.events.Items[i+1], &info)
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
				return &info, nil
			}
		}
	}
	return nil, nil
}

func (m *GCal) showCurrentEvent(info *gobar.BlockInfo) {
	t := time.Now().Add(10 * time.Minute)
	for _, item := range m.events.Items {
		endDateTime, err := time.Parse(time.RFC3339, item.End.DateTime)
		if err != nil {
			continue
		}
		if t.Before(endDateTime) {
			m.showEvent(item, info)
			return
		}
	}
	for _, item := range m.events.Items {
		endDateTime, err := time.Parse(time.RFC3339, item.End.Date)
		if err != nil {
			continue
		}
		if t.Before(endDateTime) {
			m.showEvent(item, info)
			return
		}
	}
	return
}

func (m *GCal) showEvent(event *calendar.Event, info *gobar.BlockInfo) {
	m.eventLock.Lock()
	m.currentEvent = event
	b, _ := json.Marshal(event)
	m.log.Infof("%s", b)
	m.eventLock.Unlock()
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
	return
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
	m.log.Debugf("credentials: %+v", tok)
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
