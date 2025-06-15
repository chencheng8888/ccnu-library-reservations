package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Reverser interface{
	Reverse(ctx context.Context,stuID,seatID string,startTime,endTime time.Time) error
}

type CookieGetter interface {
	GetCookie(ctx context.Context, stuID string) (string, error)
}

type reverser struct {
	cli *http.Client
	auther CookieGetter
}


func NewReverser(auther CookieGetter) Reverser {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return &reverser{
		cli: client,
		auther: auther,
	}
}

type ReverseResponse struct {
	Ret int `json:"ret"`
	Act string `json:"act"`
	Msg string `json:"msg"`
	Data interface{} `json:"data"`
	Ext interface{} `json:"ext"`
}

// 预约座位
func (r *reverser) Reverse(ctx context.Context,stuID, seatID string, startTime time.Time, endTime time.Time) error {

	cookie, err := r.auther.GetCookie(ctx, stuID)
	if err != nil {
		return fmt.Errorf("failed to get cookie: %w", err)
	}

	url := fmt.Sprintf("http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/reserve.aspx?dialogid=&dev_id=%s&lab_id=&kind_id=&room_id=&type=dev&prop=&test_id=&term=&Vnumber=&classkind=&test_name=&start=%s&end=%s&start_time=%d&end_time=%d&up_file=&memo=&act=set_resv&_=%d",
		seatID,transferTimeToURL(startTime), transferTimeToURL(endTime),transferTimeToInt(startTime),transferTimeToInt(endTime),time.Now().UnixMilli())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "http://kjyy.ccnu.edu.cn/clientweb/xcus/ic2/Default.aspx")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Cookie", cookie)
	resp, err := r.cli.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var reverseResponse ReverseResponse

	err = json.Unmarshal(bodyText, &reverseResponse)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if reverseResponse.Ret==1 {
		return nil
	}

	return fmt.Errorf("failed to reverse: %s", reverseResponse.Msg)
}


func transferTimeToURL(t time.Time) string {
	tmp := t.Format("2006-01-02 15:04")
	return url.QueryEscape(tmp)
}

func transferTimeToInt(t time.Time) int {
	return t.Hour()*100 + t.Minute()
}