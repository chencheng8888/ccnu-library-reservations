# 构建阶段
FROM golang:1.24.2-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o library-reservations .

# 运行阶段
FROM alpine:latest

# 安装 tzdata 以支持时区转换（尤其是 Asia/Shanghai）
RUN apk add --no-cache tzdata

WORKDIR /app
COPY --from=builder /app/library-reservations .


# 设置容器默认时区
ENV TZ=Asia/Shanghai

RUN mkdir "logs"
# 可选：预创建日志文件
RUN touch logs/done_tasks.log logs/failed_tasks.log

EXPOSE 15147
CMD ["/app/library-reservations"]
