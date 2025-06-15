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


const (
	FORMAT1 = "2006-01-02"
	FORMAT2 = "2006-01-02 15:04"
	FORMAT3 = "15:04"
)

type Reverser interface{
	GetSeats(ctx context.Context,stuID,roomID string,startTime,endTime time.Time) ([]SeatInfo,error)
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
		seatID,url.QueryEscape(transferTime(startTime,FORMAT2)), url.QueryEscape(transferTime(endTime,FORMAT2)),transferTimeToInt(startTime),transferTimeToInt(endTime),time.Now().UnixMilli())

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

func (r *reverser) GetSeats(ctx context.Context,stuID, roomID string, startTime time.Time, endTime time.Time) ([]SeatInfo, error) {
	
	cookie,err := r.auther.GetCookie(ctx, stuID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cookie: %w", err)
	}
	
	URL := fmt.Sprintf("http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/device.aspx?byType=devcls&classkind=8&display=fp&md=d&room_id=%s&purpose=&selectOpenAty=&cld_name=default&date=%s&fr_start=%s&fr_end=%s&act=get_rsv_sta&_=%d",
 	roomID,url.QueryEscape(transferTime(startTime,FORMAT1)),url.QueryEscape(transferTime(startTime,FORMAT3)),url.QueryEscape(transferTime(endTime,FORMAT3)),time.Now().UnixMilli())

	req, err := http.NewRequest("GET",URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Referer", "http://kjyy.ccnu.edu.cn/clientweb/xcus/ic2/Default.aspx")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Cookie", cookie)
	resp, err := r.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var getSeatResp GetSeatResp
	err = json.Unmarshal(bodyText, &getSeatResp)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	if getSeatResp.Ret != 1 {
		return nil, fmt.Errorf("failed to get available seats: %s", getSeatResp.Msg)
	}

	fmt.Println("Get available seats successfully,number of seats:", len(getSeatResp.Data))

	return getSeatResp.Data, nil
}



func transferTime(t time.Time,format string) string {
	if format==FORMAT3 {
		return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
	}
	return t.Format(format)
}

func transferTimeToInt(t time.Time) int {
	return t.Hour()*100 + t.Minute()
}


type GetSeatResp struct {
	Ret int `json:"ret"`
	Act string `json:"act"`
	Msg string `json:"msg"`
	Data []SeatInfo `json:"data"`
	Ext any `json:"ext"`
}
type Ts struct {
	ID any `json:"id"`
	Start string `json:"start"`
	End string `json:"end"`
	State string `json:"state"`
	Date any `json:"date"`
	Name any `json:"name"`
	Title string `json:"title"`
	Owner string `json:"owner"`
	Accno string `json:"accno"`
	Member string `json:"member"`
	Limit any `json:"limit"`
	Occupy bool `json:"occupy"`
}
type Ops struct {
	ID any `json:"id"`
	Start string `json:"start"`
	End string `json:"end"`
	State string `json:"state"`
	Date string `json:"date"`
	Name any `json:"name"`
	Title any `json:"title"`
	Owner any `json:"owner"`
	Accno any `json:"accno"`
	Member any `json:"member"`
	Limit int `json:"limit"`
	Occupy bool `json:"occupy"`
}
type SeatInfo struct {
	ID string `json:"id"`
	Title string `json:"title"`
	Name string `json:"name"`
	DevID string `json:"devId"`
	DevName string `json:"devName"`
	Clskind string `json:"clskind"`
	KindID string `json:"kindId"`
	KindName string `json:"kindName"`
	ClassID string `json:"classId"`
	ClassName string `json:"className"`
	LabName string `json:"labName"`
	LabID string `json:"labId"`
	RoomName string `json:"roomName"`
	RoomID int `json:"roomId"`
	BuildingID int `json:"buildingId"`
	BuildingName string `json:"buildingName"`
	Campus string `json:"campus"`
	Islong bool `json:"islong"`
	AllowLong bool `json:"allowLong"`
	Iskind bool `json:"iskind"`
	Ischeck bool `json:"ischeck"`
	Devsta int `json:"devsta"`
	Runsta int `json:"runsta"`
	State any `json:"state"`
	FreeSta int `json:"freeSta"`
	FreeTime int `json:"freeTime"`
	FreeTbl any `json:"freeTbl"`
	RuleID int `json:"ruleId"`
	Rule string `json:"rule"`
	Prop int `json:"prop"`
	Limit int `json:"limit"`
	Earliest int `json:"earliest"`
	Latest int `json:"latest"`
	Max int `json:"max"`
	Min int `json:"min"`
	Cancel int `json:"cancel"`
	MaxUser int `json:"maxUser"`
	MinUser int `json:"minUser"`
	Ext string `json:"ext"`
	Open []string `json:"open"`
	OpenStart string `json:"openStart"`
	OpenEnd string `json:"openEnd"`
	ClsDate any `json:"clsDate"`
	Ts []Ts `json:"ts"`
	Cls []any `json:"cls"`
	Ops []Ops `json:"ops"`
}

func (s SeatInfo) IfAvailable() bool {
	return s.FreeSta==0
}