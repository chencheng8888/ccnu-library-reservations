package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)


type Auther interface {
	StoreStuInfo(ctx context.Context, stuID, pwd string) error
	GetCookie(ctx context.Context,stuID string) (string, error)
}

type cookieRes struct {
	cookie string
	createdAt time.Time
}

type auther struct {
	stuInfo map[string]string // stuID -> pwd
	infoMutex sync.RWMutex

	cookies map[string]cookieRes // stuID -> cookie
	cookieMutex sync.RWMutex
}

func NewAuther() Auther {
	return &auther{
		stuInfo: make(map[string]string),
		cookies: make(map[string]cookieRes),
	}
}


func (c *auther) StoreStuInfo(ctx context.Context, stuID string, pwd string) error {
	c.infoMutex.Lock()
	defer c.infoMutex.Unlock()

	if c.stuInfo == nil {
		c.stuInfo = make(map[string]string)
	}
	c.stuInfo[stuID] = pwd
	return nil
}

func (a *auther) GetCookie(ctx context.Context, stuID string) (string, error) {
	
	a.cookieMutex.RLock()
	if cookieRes, exists := a.cookies[stuID]; exists && time.Since(cookieRes.createdAt) < 5* time.Minute {
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

func (a *auther) getCookie(ctx context.Context,stuID,pwd string) (string, error) {
	// This function should implement the logic to get the cookie from the server.
	// For now, we will return a dummy cookie.
	return "dummy_cookie", nil
}



