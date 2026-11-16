# CCNU 图书馆座位预约

华中师范大学图书馆座位预约系统的 Go 客户端，覆盖登录认证、馆舍/楼层/空间发现、空闲座位查询和预约。

客户端对接新版预约系统（`https://kjyy.ccnu.edu.cn/rem` + `https://kjyy.ccnu.edu.cn/jsq`），不复用旧版 HTTP 接口。

## 功能

- [x] 使用学号密码登录（CAS → Handoff Token → Session Token）
- [x] 查询馆舍、楼层和可选日期
- [x] 按 Reservation Window 查询 Space 的空闲座位数
- [x] 查询整个 Reservation Window 内空闲的 Free Seats
- [x] 预约座位
- [x] 取消预约

## 安装

```bash
go get github.com/chencheng8888/ccnu-library-reservations
```

需要 Go 1.24 或更高版本。

## 快速开始

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	library "github.com/chencheng8888/ccnu-library-reservations"
)

func main() {
	ctx := context.Background()
	stuID, password := "你的学号", "你的密码"

	auth := library.NewAuthenticator()
	if err := auth.StoreStuInfo(ctx, stuID, password); err != nil {
		log.Fatal(err)
	}
	reserver := library.NewReserver(auth)

	// 1. 查询馆舍、楼层和可选日期
	catalog, err := reserver.Catalog(ctx, stuID)
	if err != nil {
		log.Fatal(err)
	}
	for _, building := range catalog.Buildings {
		fmt.Println(building.Name)
		for _, floor := range building.Floors {
			fmt.Println("  -", floor.Name, floor.ID)
		}
	}

	// 2. 构造明天的 Reservation Window
	loc, _ := time.LoadLocation("Asia/Shanghai")
	day := time.Now().In(loc).AddDate(0, 0, 1)
	start := time.Date(day.Year(), day.Month(), day.Day(), 12, 30, 0, 0, loc)
	end := time.Date(day.Year(), day.Month(), day.Day(), 21, 30, 0, 0, loc)
	window, err := library.NewWindow(start, end)
	if err != nil {
		log.Fatal(err)
	}

	// 3. 查看楼层内各 Space 的空闲座位数
	space := library.Spaces["n1m"]
	spaces, err := reserver.ListSpaces(ctx, stuID, space.BuildingID, space.FloorID, window)
	if err != nil {
		log.Fatal(err)
	}
	for _, item := range spaces {
		fmt.Printf("%s: %d/%d free\n", item.Name, item.SeatFree, item.SeatTotal)
	}

	// 4. 查询 Free Seats
	seats, err := reserver.ListFreeSeats(ctx, stuID, space.ID, window)
	if err != nil {
		log.Fatal(err)
	}
	if len(seats) == 0 {
		log.Fatal("no Free Seats for the window")
	}

	// 5. 预约（会真实下单；确认座位后再取消注释）
	// reservation, err := reserver.Reserve(ctx, stuID, seats[0].ID, window)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println("Reservation ID:", reservation.ID)
}
```

完整示例见 [`example/example.go`](example/example.go)：

```bash
go run example/example.go -stuId=你的学号 -password=你的密码
```

默认是只读演练；加上 `-reserve` 才会真实创建预约。

## 核心接口

### Authenticator

```go
type Authenticator interface {
	StoreStuInfo(ctx context.Context, stuID, pwd string) error
	SessionToken(ctx context.Context, stuID string) (string, error)
}
```

`StoreStuInfo` 只保存凭据。首次调用 `SessionToken` 时才执行完整的登录流程，之后的调用复用 Session Token；过期后自动重新登录。

### Reserver

```go
type Reserver interface {
	Catalog(ctx context.Context, stuID string) (Catalog, error)
	ListSpaces(ctx context.Context, stuID, buildingID, floorID string, window Window) ([]SpaceAvailability, error)
	ListFreeSeats(ctx context.Context, stuID, spaceID string, window Window) ([]FreeSeat, error)
	Reserve(ctx context.Context, stuID, seatID string, window Window) (Reservation, error)
	Cancel(ctx context.Context, stuID, reservationID string) error
}
```

## 领域模型

术语的完整定义见 [`GLOSSARY.md`](GLOSSARY.md)。

| 类型 | 含义 |
| --- | --- |
| `Catalog` | 学生可用的 Building、Floor 和日期 |
| `Building` / `Floor` | 馆舍及其楼层 |
| `Space` | 楼层内可预约的区域，包含若干 Seat |
| `Seat` | 一个可预约的座位 |
| `FreeSeat` | 在整个 Reservation Window 内都空闲的 Seat |
| `Window` | 日期 + 当天分钟数区间，由 `NewWindow` 构造 |
| `Reservation` | 学生对某个 Seat 在某个 Window 内的占用 |

### Space Code

`Spaces` 是常用 Space 的静态别名表，方便从旧代码迁移；运行时以 `Catalog` / `ListSpaces` 的发现结果为准。

| Space Code | Space ID | 名称 |
| --- | --- | --- |
| `n1` | `1995338268580167680` | 南湖分馆一楼开敞座位区 |
| `n1m` | `1995338594917990400` | 南湖分馆一楼中庭开敞座位区 |
| `n2` | `1995338850321743872` | 南湖分馆二楼开敞座位区 |

## 预约规则

`Reserve` 会先从系统配置读取 Step（最小时间粒度）和 Max Duration（单次最长时长），并校验 Reservation Window：

- 日期必须存在，起止分钟必须在同一天内；
- 开始必须早于结束；
- 起止分钟必须对齐到 Step；
- 时长不得超过 Max Duration。

非法窗口返回可识别的 `ErrInvalidWindow`。上游拒绝的请求返回 `*APIError`，Session 过期返回 `ErrSessionExpired` 并自动重试一次。

## 测试

单元测试不访问网络。集成测试从仓库根目录的 `.env` 读取凭据，缺少配置时自动跳过：

```bash
cat > .env <<'EOF'
STUID=你的学号
PASSWORD=你的密码
EOF
go test ./...
```

`.env` 已在 `.gitignore` 中忽略。

真实预约测试会创建并立即取消一个预约，默认跳过，只有显式设置环境变量才会运行：

```bash
REAL_BOOKING=1 go test -run TestReserveAndCancelRealBooking -v
```

## 安全说明

1. 不要提交 `.env` 或硬编码学号密码；
2. 避免高频请求，以免给图书馆系统造成压力；
3. 遵守图书馆的预约规则和使用政策；
4. 所有时间均按 `Asia/Shanghai` 处理。

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE)。

## 免责声明

本项目仅供学习和研究使用。开发者不对因使用本项目产生的任何问题负责。
