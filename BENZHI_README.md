# BENZHI_README

## 项目说明
- 项目：benzhi-project-25486d65-78cb-41a8-93f4-88653190861f
- 项目用途：展柜环境风险巡检闭环工作台，支持环境采集、规则评估、专家复核、整改复查与审计归档。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：展柜环境风险巡检闭环
- 项目概述：面向博物馆保管员和文保专家的展柜环境风险巡检工作台，将传感读数与现场观察汇聚为可复核的风险结论，推动整改证据提交并在复查通过后归档完整时间线。
- 核心工作流：巡检批次建立→采集展柜环境与观察记录→规则评估风险→专家复核锁定结论→生成整改任务→提交证据并复查→通过后归档
- 对外接口：Go 服务提供原生 HTML、CSS 和 JavaScript 的浏览器工作台，支持 -addr=127.0.0.1:<port> 或 PORT 环境变量，默认监听 127.0.0.1:19081

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/vitrinemon -addr=127.0.0.1:19081 -self-check
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-25486d65-78cb-41a8-93f4-88653190861f-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-25486d65-78cb-41a8-93f4-88653190861f-arm64 linux/arm64
docker run -it benzhi-project-25486d65-78cb-41a8-93f4-88653190861f-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/vitrinemon -addr=127.0.0.1:19081 -self-check`
