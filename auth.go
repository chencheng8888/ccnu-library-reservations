package library_reservation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Authenticator stores student credentials and produces Session Tokens.
type Authenticator interface {
	StoreStuInfo(ctx context.Context, stuID, pwd string) error
	SessionToken(ctx context.Context, stuID string) (string, error)
}

// NewAuthenticator creates an Authenticator.
func NewAuthenticator() Authenticator {
	return newAuthenticator()
}

const (
	sessionTTL               = 5 * time.Minute
	seatReservationProjectID = "1981289670190006272"
)

type sessionToken struct {
	token     string
	createdAt time.Time
}

type authenticator struct {
	mu       sync.Mutex
	stuInfo  map[string]string
	sessions map[string]sessionToken
	api      *apiClient
}

func newAuthenticator() *authenticator {
	return &authenticator{
		stuInfo:  make(map[string]string),
		sessions: make(map[string]sessionToken),
		api:      newAPIClient(),
	}
}

func (a *authenticator) StoreStuInfo(ctx context.Context, stuID, pwd string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stuInfo[stuID] = pwd
	return nil
}

func (a *authenticator) SessionToken(ctx context.Context, stuID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if session, ok := a.sessions[stuID]; ok && time.Since(session.createdAt) < sessionTTL {
		return session.token, nil
	}
	pwd, ok := a.stuInfo[stuID]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownStudent, stuID)
	}
	token, err := a.login(ctx, stuID, pwd)
	if err != nil {
		return "", err
	}
	a.sessions[stuID] = sessionToken{token: token, createdAt: time.Now()}
	return token, nil
}

func (a *authenticator) invalidate(stuID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, stuID)
}

func (a *authenticator) login(ctx context.Context, stuID, pwd string) (string, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return "", fmt.Errorf("create cookie jar: %w", err)
	}
	client := &http.Client{Jar: jar, Timeout: 25 * time.Second}

	// Entering the new system redirects through /rem to the CAS login page.
	resp, err := get(ctx, client, remBaseURL+"/static/sso/webOAuthRed")
	if err != nil {
		return "", fmt.Errorf("open cas login: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("read cas login page: %w", err)
	}
	if resp.Request.URL.Host != "account.ccnu.edu.cn" {
		return "", fmt.Errorf("unexpected login page: %s", resp.Request.URL)
	}

	lt, execution, action, err := parseCASLoginForm(body)
	if err != nil {
		return "", err
	}
	actionURL, err := resp.Request.URL.Parse(strings.ReplaceAll(action, "&amp;", "&"))
	if err != nil {
		return "", fmt.Errorf("parse cas form action: %w", err)
	}

	form := url.Values{
		"username":  {stuID},
		"password":  {pwd},
		"lt":        {lt},
		"execution": {execution},
		"_eventId":  {"submit"},
		"submit":    {"登录"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, actionURL.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("create cas login request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://account.ccnu.edu.cn")
	req.Header.Set("Referer", actionURL.String())
	req.Header.Set("User-Agent", userAgent)

	postResp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("submit cas login: %w", err)
	}
	projectBody, err := io.ReadAll(postResp.Body)
	postResp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("read login result: %w", err)
	}
	if postResp.Request.URL.Host == "account.ccnu.edu.cn" {
		return "", ErrInvalidCredentials
	}

	projectID := findSeatProjectID(projectBody)
	if projectID == "" {
		projectID = seatReservationProjectID
	}

	noRedirect := &http.Client{
		Jar:     jar,
		Timeout: 25 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	gotoResp, err := get(ctx, noRedirect, remBaseURL+"/static/gotoProject/"+projectID)
	if err != nil {
		return "", fmt.Errorf("open seat reservation project: %w", err)
	}
	gotoResp.Body.Close()
	if gotoResp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("unexpected project redirect status %d", gotoResp.StatusCode)
	}
	handoffToken, err := tokenFromLocation(gotoResp.Header.Get("Location"))
	if err != nil {
		return "", err
	}

	exchangeURL := jsqBaseURL + "/static/public/auth/cas/" + url.PathEscape(handoffToken)
	env, err := a.api.postEnvelope(ctx, exchangeURL, map[string]string{
		"token":     handoffToken,
		"loginType": "PC",
	}, "")
	if err != nil {
		return "", fmt.Errorf("exchange handoff token: %w", err)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := decodeEnvelopeData(env, &data); err != nil {
		return "", fmt.Errorf("decode session token: %w", err)
	}
	if data.Token == "" {
		return "", errors.New("exchange handoff token: empty session token")
	}
	return data.Token, nil
}

func get(ctx context.Context, client *http.Client, endpoint string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("User-Agent", userAgent)
	return client.Do(req)
}

func parseCASLoginForm(body []byte) (lt, execution, action string, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return "", "", "", fmt.Errorf("parse cas login page: %w", err)
	}
	lt, _ = doc.Find("input[name='lt']").Attr("value")
	execution, _ = doc.Find("input[name='execution']").Attr("value")
	action, _ = doc.Find("form#fm1").Attr("action")
	if action == "" {
		action, _ = doc.Find("form").First().Attr("action")
	}
	if lt == "" || execution == "" || action == "" {
		return "", "", "", errors.New("cas login form not recognised")
	}
	return lt, execution, action, nil
}

var gotoProjectPattern = regexp.MustCompile(`/rem/static/gotoProject/(\d+)`)

func findSeatProjectID(body []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var id string
	doc.Find("[onclick]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if !strings.Contains(s.Text(), "座位预约") {
			return true
		}
		onclick, _ := s.Attr("onclick")
		if match := gotoProjectPattern.FindStringSubmatch(onclick); len(match) == 2 {
			id = match[1]
			return false
		}
		return true
	})
	if id != "" {
		return id
	}
	doc.Find("a[href]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if !strings.Contains(s.Text(), "座位预约") {
			return true
		}
		href, _ := s.Attr("href")
		if match := gotoProjectPattern.FindStringSubmatch(href); len(match) == 2 {
			id = match[1]
			return false
		}
		return true
	})
	return id
}

func tokenFromLocation(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("parse handoff location: %w", err)
	}
	if token := u.Query().Get("token"); token != "" {
		return token, nil
	}
	fragment := u.Fragment
	if i := strings.Index(fragment, "?"); i >= 0 {
		query, err := url.ParseQuery(fragment[i+1:])
		if err == nil && query.Get("token") != "" {
			return query.Get("token"), nil
		}
	}
	return "", fmt.Errorf("handoff token not found in %q", location)
}
