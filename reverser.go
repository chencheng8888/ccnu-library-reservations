package library_reservation

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"libary-reservations/pkg"
	"net/http"
	"net/url"
	"sort"
	"time"
)

type Reverser interface {
	GetSeatsByTime(ctx context.Context, stuID, roomID string, startTime, endTime time.Time, onlyAvailable bool) ([]Seat, error)
	Reverse(ctx context.Context, stuID, seatID string, startTime, endTime time.Time) error
}

type reverser struct {
	cli *http.Client
	au  Auther
}

func NewReverser(au Auther) Reverser {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	return &reverser{
		cli: client,
		au:  au,
	}
}

type ReverseResponse struct {
	Ret  int         `json:"ret"`
	Act  string      `json:"act"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
	Ext  interface{} `json:"ext"`
}

// Reverse 预约座位
func (r *reverser) Reverse(ctx context.Context, stuID, seatID string, startTime time.Time, endTime time.Time) error {

	cookie, err := r.au.GetCookie(ctx, stuID)
	if err != nil {
		return fmt.Errorf("failed to get cookie: %w", err)
	}

	reverseURL := fmt.Sprintf("http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/reserve.aspx?dialogid=&dev_id=%s&lab_id=&kind_id=&room_id=&type=dev&prop=&test_id=&term=&Vnumber=&classkind=&test_name=&start=%s&end=%s&start_time=%d&end_time=%d&up_file=&memo=&act=set_resv&_=%d",
		seatID, url.QueryEscape(pkg.TransferTimeToString(startTime, pkg.FORMAT2)), url.QueryEscape(pkg.TransferTimeToString(endTime, pkg.FORMAT2)), transferTimeToInt(startTime), transferTimeToInt(endTime), time.Now().UnixMilli())

	req, err := http.NewRequest("GET", reverseURL, nil)
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

	if reverseResponse.Ret == 1 {
		return nil
	}

	return fmt.Errorf("failed to reverse: %s", reverseResponse.Msg)
}

func (r *reverser) GetSeatsByTime(ctx context.Context, stuID, roomID string, startTime time.Time, endTime time.Time, onlyAvailable bool) ([]Seat, error) {
	cseats, err := r.getSeats(ctx, stuID, roomID, startTime, endTime)
	if err != nil {
		return nil, err
	}

	seats := transferCrawSeat(cseats, startTime, endTime)

	if !onlyAvailable {
		return seats, err
	}

	var availableSeats []Seat
	for _, seat := range seats {
		if free, _ := seat.IsFree(startTime, endTime); free {
			availableSeats = append(availableSeats, seat)
		}
	}
	if len(availableSeats) == 0 {
		return nil, fmt.Errorf("no available seats found in the specified time range")
	}
	return availableSeats, nil
}

func (r *reverser) getSeats(ctx context.Context, stuID, roomID string, startTime time.Time, endTime time.Time) ([]crawSeatInfo, error) {
	cookie, err := r.au.GetCookie(ctx, stuID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cookie: %w", err)
	}

	URL := fmt.Sprintf("http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/device.aspx?byType=devcls&classkind=8&display=fp&md=d&room_id=%s&purpose=&selectOpenAty=&cld_name=default&date=%s&fr_start=%s&fr_end=%s&act=get_rsv_sta&_=%d",
		roomID, url.QueryEscape(pkg.TransferTimeToString(startTime, pkg.FORMAT1)), url.QueryEscape(pkg.TransferTimeToString(startTime, pkg.FORMAT3)), url.QueryEscape(pkg.TransferTimeToString(endTime, pkg.FORMAT3)), time.Now().UnixMilli())

	req, err := http.NewRequest("GET", URL, nil)
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

	var getSeatResp getSeatResp
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

type getSeatResp struct {
	Ret  int            `json:"ret"`
	Act  string         `json:"act"`
	Msg  string         `json:"msg"`
	Data []crawSeatInfo `json:"data"`
	Ext  any            `json:"ext"`
}
type ts struct {
	ID     any    `json:"id"`
	Start  string `json:"start"`
	End    string `json:"end"`
	State  string `json:"state"`
	Date   any    `json:"date"`
	Name   any    `json:"name"`
	Title  string `json:"title"`
	Owner  string `json:"owner"`
	Accno  string `json:"accno"`
	Member string `json:"member"`
	Limit  any    `json:"limit"`
	Occupy bool   `json:"occupy"`
}
type ops struct {
	ID     any    `json:"id"`
	Start  string `json:"start"`
	End    string `json:"end"`
	State  string `json:"state"`
	Date   string `json:"date"`
	Name   any    `json:"name"`
	Title  any    `json:"title"`
	Owner  any    `json:"owner"`
	Accno  any    `json:"accno"`
	Member any    `json:"member"`
	Limit  int    `json:"limit"`
	Occupy bool   `json:"occupy"`
}
type crawSeatInfo struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Name         string   `json:"name"`
	DevID        string   `json:"devId"`
	DevName      string   `json:"devName"`
	Clskind      string   `json:"clskind"`
	KindID       string   `json:"kindId"`
	KindName     string   `json:"kindName"`
	ClassID      string   `json:"classId"`
	ClassName    string   `json:"className"`
	LabName      string   `json:"labName"`
	LabID        string   `json:"labId"`
	RoomName     string   `json:"roomName"`
	RoomID       int      `json:"roomId"`
	BuildingID   int      `json:"buildingId"`
	BuildingName string   `json:"buildingName"`
	Campus       string   `json:"campus"`
	Islong       bool     `json:"islong"`
	AllowLong    bool     `json:"allowLong"`
	Iskind       bool     `json:"iskind"`
	Ischeck      bool     `json:"ischeck"`
	Devsta       int      `json:"devsta"`
	Runsta       int      `json:"runsta"`
	State        any      `json:"state"`
	FreeSta      int      `json:"freeSta"`
	FreeTime     int      `json:"freeTime"`
	FreeTbl      any      `json:"freeTbl"`
	RuleID       int      `json:"ruleId"`
	Rule         string   `json:"rule"`
	Prop         int      `json:"prop"`
	Limit        int      `json:"limit"`
	Earliest     int      `json:"earliest"`
	Latest       int      `json:"latest"`
	Max          int      `json:"max"`
	Min          int      `json:"min"`
	Cancel       int      `json:"cancel"`
	MaxUser      int      `json:"maxUser"`
	MinUser      int      `json:"minUser"`
	Ext          string   `json:"ext"`
	Open         []string `json:"open"`
	OpenStart    string   `json:"openStart"`
	OpenEnd      string   `json:"openEnd"`
	ClsDate      any      `json:"clsDate"`
	Ts           []ts     `json:"ts"`
	Cls          []any    `json:"cls"`
	Ops          []ops    `json:"ops"`
}

func transferCrawSeat(infos []crawSeatInfo, reserveStartTime, reserveEndTime time.Time) []Seat {
	seats := make([]Seat, 0, len(infos))

	for _, info := range infos {
		var occupyStates []Period
		for _, t := range info.Ts {
			startTime, _ := pkg.TransferStringToTime(t.Start, pkg.FORMAT2)
			endTime, _ := pkg.TransferStringToTime(t.End, pkg.FORMAT2)

			if startTime.After(reserveEndTime) {
				continue
			}
			if endTime.Before(reserveStartTime) {
				continue
			}

			occupyStates = append(occupyStates, Period{
				Owner:     t.Owner,
				StartTime: pkg.MaxTime(startTime, reserveStartTime),
				EndTime:   pkg.MinTime(endTime, reserveEndTime),
			})
		}

		// 对占用状态进行排序
		sort.Slice(occupyStates, func(i, j int) bool {
			if occupyStates[i].StartTime.Equal(occupyStates[j].StartTime) {
				return occupyStates[i].EndTime.Before(occupyStates[j].EndTime)
			}
			return occupyStates[i].StartTime.Before(occupyStates[j].StartTime)
		})

		roomID := fmt.Sprintf("%d", info.RoomID)

		seat := NewSeat(info.DevID, info.DevName, roomID, info.RoomName, reserveStartTime,
			reserveEndTime, info.FreeSta == 0, occupyStates)
		seats = append(seats, seat)
	}
	return seats
}
