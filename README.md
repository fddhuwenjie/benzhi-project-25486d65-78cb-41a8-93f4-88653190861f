# 展柜环境风险巡检闭环

这是面向博物馆保管员、文保专家和设施整改负责人的浏览器工作台。系统把展柜温湿度、照度读数与现场观察汇聚起来，按材质阈值评估风险，经专家复核后生成整改任务，接收证据并在复查通过时归档完整审计时间线。

## 构建与运行

需要 Go 1.22 或更高版本。标准命令如下：

```text
go test ./...
go run ./cmd/vitrinemon -addr=127.0.0.1:19081
```

默认服务地址为 `127.0.0.1:19081`，也可以使用 `-addr=127.0.0.1:<port>` 或设置 `PORT` 端口号。浏览器访问 `/inspection`。运行时数据保存在 `.vitrinemon` 目录，批次快照使用临时文件原子替换，审计事件追加写入 JSON Lines 文件。

## 自检

以下命令会启动短生命周期服务，探测健康检查和核心页面后自动退出：

```text
go run ./cmd/vitrinemon -addr=127.0.0.1:19081 -self-check
```

HTTP API 使用 `X-Request-ID` 保证重复提交幂等，并在写入时检查批次修订号。除基础批次、观察、复核、整改和归档接口外，还提供 `POST /api/batches/adjust`、`POST /api/observations/import`、`POST /api/observations/correct`、`POST /api/observations/revoke`、`GET /api/risk-stats`、`GET /api/tasks`、`POST /api/evidence`、`GET /api/reviews` 和 `GET /api/archive/export`。批量导入支持 JSON 行集合和 CSV 文件，返回逐行成功或中文失败原因。

完整性检查、规则重算、任务治理和归档检索分别通过 `GET /api/integrity`、`POST /api/rules/preview`、`POST /api/rules/apply`、`POST /api/tasks/govern`、`GET /api/archive/search` 提供；这些接口均返回当前修订号并使用请求标识保证幂等，复核提交会自动执行完整性校验。
