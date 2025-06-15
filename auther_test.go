package main

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

func LoadInfo() (string,string) {
	 // 加载 .env 文件
    err := godotenv.Load()
    if err != nil {
        return "",""
    }
	stuID := os.Getenv("STUID")
    pwd := os.Getenv("PASSWORD")
	return stuID, pwd
}

func TestGetCookie(t *testing.T) {
	
	stuID, pwd := LoadInfo()
	if stuID == "" || pwd == "" {
		t.Fatal("STUID or PASSWORD not set in .env file")
	}

	auther := NewAuther()
	err := auther.StoreStuInfo(context.Background(),stuID, pwd)
	if err!=nil {
		t.Fatalf("failed to store student info: %v", err)
	}

	cookie,err := auther.GetCookie(context.Background(), stuID)
	if err != nil {
		t.Fatalf("failed to get cookie: %v", err)
	}
	if cookie == "" {
		t.Fatal("cookie should not be empty")
	}
	t.Logf("Cookie for student ID %s: %s", stuID, cookie)
}