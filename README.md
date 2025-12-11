# CCNU 图书馆座位预约系统

一个用于华中师范大学图书馆座位预约的 Go 语言库，提供自动化预约、座位查询和用户认证功能。

## 主要功能
- [x] 添加用户(添加学号和密码)
- [x] 查看座位
- [x] 预约座位

## 使用

### 前提条件

-   Go 1.24.2 或更高版本

### 安装步骤

## 引用
```bash
go get github.com/chencheng8888/ccnu-library-reservations
```

## 快速开始

### 基本使用示例

参考 `example/example.go` 文件：

```go
package main

import (
	"context"
	"flag"
	"fmt"
	libraryreservation "github.com/chencheng8888/ccnu-library-reservations"
	"math/rand"
	"time"
)

var (
	stuId    string
	password string
)

func init() {
	flag.StringVar(&stuId, "stuId", "your_stuId", "stuId")
	flag.StringVar(&password, "password", "your_password", "password")
}

func main() {

	flag.Parse()

	ctx := context.Background()
	auth := libraryreservation.NewAuther()
	r := libraryreservation.NewReverser(auth)

	err := auth.StoreStuInfo(ctx, stuId, password)
	if err != nil {
		panic(err)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}

	now := time.Now().In(loc)
	tomorrow := now.AddDate(0, 0, 1)

	// 创建明天 12:30 的时间
	tomorrow14 := time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		12, 30, 0, 0,
		tomorrow.Location(),
	)

	// 创建明天 21:30 的时间
	tomorrow21 := time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
		21, 30, 0, 0,
		tomorrow.Location(),
	)

	seats, err := r.GetSeatsByTime(ctx, stuId, libraryreservation.Rooms["n1m"], tomorrow14, tomorrow21, true)
	if err != nil {
		panic(err)
	}

	if len(seats) == 0 {
		panic("no available seats found")
	}

	fmt.Println("Available seats cnt:", len(seats))

	// 先用当前时间设置随机种子，避免每次运行结果相同
	rand.Seed(time.Now().UnixNano())

	// 随机选择索引
	idx := rand.Intn(len(seats))

	fmt.Printf("Seat: %+v\n", seats[idx])

	err = r.Reverse(ctx, stuId, seats[idx].SeatID, tomorrow14, tomorrow21)
	if err != nil {
		panic(err)
	}
}

```

### 运行示例

```bash
go run example/example.go -stuId=你的学号 -password=你的密码
```

## API 文档

### 核心接口

#### Auther 接口

```go
type Auther interface {
    StoreStuInfo(ctx context.Context, stuID, pwd string) error
    GetCookie(ctx context.Context, stuID string) (string, error)
}
```

#### Reverser 接口

```go
type Reverser interface {
    GetSeatsByTime(ctx context.Context, stuID, roomID string, startTime, endTime time.Time, onlyAvailable bool) ([]Seat, error)
    Reverse(ctx context.Context, stuID, seatID string, startTime, endTime time.Time) error
}
```



### 数据结构

#### Seat 结构

```go
type Seat struct {
    SeatID            string    // 座位ID
    SeatName          string    // 座位名称
    RoomID            string    // 区域ID
    RoomName          string    // 区域名称
    OccupyStates      []Period  // 占用状态
    ReserveStartTime  time.Time // 预定的开始时间
    ReserveEndTime    time.Time // 预定的结束时间
    isFreeInTimeRange bool      // 在预定时间段内是否空闲
}
```

#### Period 结构

```go
type Period struct {
    Owner     string
    StartTime time.Time
    EndTime   time.Time
}
```

### 可用房间

项目预定义了以下房间 ID：

| 房间代码 | 房间 ID   | 描述                       |
| -------- | --------- | -------------------------- |
| n1       | 101699179 | 南湖分馆一楼开敞座位区     |
| n1m      | 101699187 | 南湖分馆一楼中庭开敞座位区 |
| n2       | 101699189 | 南湖分馆二楼开敞座位区     |

使用方式：

```go
roomID := library_reservation.Rooms["n1m"]
```

## 高级用法

### 自定义预约逻辑

```go
// 查询所有座位（包括已被占用的）
seats, err := reverser.GetSeatsByTime(ctx, stuId, roomID, startTime, endTime, false)

// 分析每个座位的空闲时间段
for _, seat := range seats {
    isFree, freePeriods := seat.IsFree(startTime, endTime)
    if isFree {
        fmt.Printf("座位 %s 完全空闲\n", seat.SeatName)
    } else if len(freePeriods) > 0 {
        fmt.Printf("座位 %s 部分空闲，空闲时间段:\n", seat.SeatName)
        for _, period := range freePeriods {
            fmt.Printf("  %s - %s\n",
                period.StartTime.Format("15:04"),
                period.EndTime.Format("15:04"))
        }
    }
}
```

### 批量预约

```go
// 为多个用户预约座位
users := []struct {
    stuId    string
    password string
}{
    {"学号1", "密码1"},
    {"学号2", "密码2"},
}

for _, user := range users {
    err := auth.StoreStuInfo(ctx, user.stuId, user.password)
    if err != nil {
        log.Printf("存储用户 %s 信息失败: %v", user.stuId, err)
        continue
    }

    seats, err := reverser.GetSeatsByTime(ctx, user.stuId, roomID, startTime, endTime, true)
    if err != nil || len(seats) == 0 {
        log.Printf("用户 %s 无可用座位", user.stuId)
        continue
    }

    // 选择第一个可用座位
    err = reverser.Reverse(ctx, user.stuId, seats[0].SeatID, startTime, endTime)
    if err != nil {
        log.Printf("用户 %s 预约失败: %v", user.stuId, err)
    } else {
        log.Printf("用户 %s 预约成功", user.stuId)
    }
}
```


## 注意事项
1. **安全性**：请妥善保管学号和密码，不要在公共代码库中硬编码
2. **使用频率**：避免频繁请求，以免对图书馆系统造成压力
3. **遵守规定**：请遵守图书馆的使用规定和预约政策
4. **时区设置**：所有时间操作均使用 Asia/Shanghai 时区

## 故障排除

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情

## 免责声明

本项目仅供学习和研究使用，请遵守华中师范大学图书馆的相关规定。开发者不对因使用本项目而产生的任何问题负责。


---

**提示**：使用前请确保你已经阅读并理解图书馆的预约规则，合理使用预约系统。
