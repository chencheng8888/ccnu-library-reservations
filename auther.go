package library_reservation

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	CookieKey1 = "ASP.NET_SessionId"
	CookieKey2 = "JSESSIONID"
)

type Auther interface {
	StoreStuInfo(ctx context.Context, stuID, pwd string) error
	GetCookie(ctx context.Context, stuID string) (string, error)
}

type cookieRes struct {
	cookie    string
	createdAt time.Time
}

type auther struct {
	stuInfo   map[string]string // stuID -> pwd
	infoMutex sync.RWMutex

	cookies     map[string]cookieRes // stuID -> cookie
	cookieMutex sync.RWMutex
}

func NewAuther() Auther {
	return &auther{
		stuInfo: make(map[string]string),
		cookies: make(map[string]cookieRes),
	}
}

func (a *auther) StoreStuInfo(ctx context.Context, stuID string, pwd string) error {
	a.infoMutex.Lock()
	defer a.infoMutex.Unlock()

	if a.stuInfo == nil {
		a.stuInfo = make(map[string]string)
	}
	a.stuInfo[stuID] = pwd
	return nil
}

//func (a *au) GetStuIDs(ctx context.Context) []string {
//	a.infoMutex.RLock()
//	defer a.infoMutex.RUnlock()
//
//	var stuIDs []string
//	for stuID := range a.stuInfo {
//		stuIDs = append(stuIDs, stuID)
//	}
//	return stuIDs
//}

func (a *auther) GetCookie(ctx context.Context, stuID string) (string, error) {

	a.cookieMutex.RLock()
	if cookieRes, exists := a.cookies[stuID]; exists && time.Since(cookieRes.createdAt) < 5*time.Minute {
		a.cookieMutex.RUnlock()
		return cookieRes.cookie, nil
	}
	a.cookieMutex.RUnlock()

	a.cookieMutex.Lock()
	defer a.cookieMutex.Unlock()

	if pwd, exists := a.stuInfo[stuID]; exists {
		cookie, err := a.getCookie(ctx, stuID, pwd)
		if err != nil {
			return "", err
		}
		a.cookies[stuID] = cookieRes{cookie: cookie, createdAt: time.Now()}
		return cookie, nil
	}

	return "", fmt.Errorf("student ID %s not found", stuID)
}

func (a *auther) getCookie(ctx context.Context, stuID, pwd string) (string, error) {

	cli, infos, err := a.getNecessaryInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get necessary info: %w", err)
	}

	err = a.login(ctx, cli, stuID, pwd, infos["lt"], infos["execution"])
	if err != nil {
		return "", fmt.Errorf("failed to login: %w", err)
	}

	return CookieKey1 + "=" + infos[CookieKey1], nil
}

func (a *auther) getNecessaryInfo(ctx context.Context) (*http.Client, map[string]string, error) {
	infos := make(map[string]string)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	jar, _ := cookiejar.New(nil)

	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			fmt.Println("Redirected to:", req.URL)
			return nil // 允许重定向，模拟浏览器自动跳转
		},
		Transport: tr,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://kjyy.ccnu.edu.cn/clientweb/xcus/ic2/Default.aspx?version=3.00.20181109", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	lt, _ := doc.Find("input[name='lt']").Attr("value")
	execution, _ := doc.Find("input[name='execution']").Attr("value")
	if lt == "" || execution == "" {
		return nil, nil, fmt.Errorf("failed to find lt or execution in the response")
	}

	infos["lt"] = lt
	infos["execution"] = execution

	domains := []string{
		"http://kjyy.ccnu.edu.cn",
		"https://account.ccnu.edu.cn/cas",
	}

	var getCookieKey1, getCookieKey2 bool

	for _, domain := range domains {
		rootURL, _ := url.Parse(domain)
		for _, cookie := range jar.Cookies(rootURL) {
			if cookie.Name == CookieKey1 {
				getCookieKey1 = true
				infos[cookie.Name] = cookie.Value
			}
			if cookie.Name == CookieKey2 {
				getCookieKey2 = true
				infos[cookie.Name] = cookie.Value
			}
			fmt.Println("Cookie:", cookie.Name, "Value:", cookie.Value, "Domain:", cookie.Domain)
		}
	}

	if !getCookieKey1 || !getCookieKey2 {
		return nil, nil, fmt.Errorf("failed to get cookies, expected 2 cookies")
	}

	fmt.Printf("necessary info: %+v\n", infos)

	return client, infos, nil
}

func (a *auther) login(ctx context.Context, client *http.Client, stuID, pwd, lt, execution string) error {
	var redirected bool

	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		redirected = true
		return nil
	}

	//登录
	form := url.Values{
		"username":  {stuID},
		"password":  {pwd},
		"lt":        {lt},
		"execution": {execution},
		"_eventId":  {"submit"},
		"submit":    {"登录"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://account.ccnu.edu.cn/cas/login?service=http://kjyy.ccnu.edu.cn/loginall.aspx?page=", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}

	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://account.ccnu.edu.cn")
	req.Header.Set("Referer", "https://account.ccnu.edu.cn/cas/login?service=http://kjyy.ccnu.edu.cn/loginall.aspx?page=")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	req.Header.Set("sec-ch-ua", `"Google Chrome";v="137", "Chromium";v="137", "Not/A)Brand";v="24"`)
	req.Header.Set("sec-ch-ua-mobile", "?0")
	req.Header.Set("sec-ch-ua-platform", `"Windows"`)

	_, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("send request failed: %w", err)
	}

	// 检查是否重定向
	// 如果没有，则代表失败
	if !redirected {
		return fmt.Errorf("login did not redirect, check your stuID and password")
	}

	return nil
}
