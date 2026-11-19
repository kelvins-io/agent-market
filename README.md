# Agent Market

基于 [Eino](https://github.com/cloudwego/eino) ADK 与 [eino-ext/a2a](https://github.com/cloudwego/eino-ext/tree/main/a2a) 的本地 **Agent 市场**演示：附近 Agent 可自注册、按标签/分类/关键词/地理距离被发现，并通过 A2A（JSON-RPC）互相调用；人可通过 Web UI 与 Market Host 或指定 Agent 对话。

## 功能概览

| 能力 | 说明 |
|------|------|
| Agent 注册 / 心跳 | 示例 Agent 启动后向 Market 登记，周期心跳；超时视为 offline |
| 多维发现 | 分类精确匹配 ∩ 标签任一命中 ∩ 关键词 ∩ 半径（Haversine），按距离升序 |
| 地理 | 浏览器定位；默认北京坐标；`GET /api/v1/geo/reverse` 反查地名 |
| Market Host | Eino ChatModelAgent，工具 `discover_agents` / `talk_to_agent`，SSE 流式回复 |
| A2A 代理 | 人 → Market → 目标 Agent；兼容 **eino-ext 旧协议** 与 **a2a-go v2**（可选 Bearer） |
| 离线演示 | `MOCK_LLM=1` 时不访问外部模型，仍可走发现与工具编排 |
| Web UI | 定位、搜索、语音（浏览器 ASR）、Host 对话、直连 Agent |
| 容器化 | 单镜像双进程：`market`（API + 静态页）+ `agents`（四个示例 Agent） |

## 架构

```
┌─────────────┐     HTTP/SSE      ┌──────────────────────┐
│  Web UI     │ ───────────────► │  Market Gateway      │
│  (React)    │                  │  cmd/market          │
└─────────────┘                  │  · Registry (SQLite) │
                                 │  · Discovery         │
                                 │  · Host Agent        │
                                 │  · A2A 代理          │
                                 └──────────┬───────────┘
                                            │ A2A JSON-RPC
                    ┌───────────────────────┼───────────────────────┐
                    ▼                       ▼                       ▼
              Weather :8101          LocalGuide :8102         … MediaTV :8104
              (Hertz + eino-ext/a2a，自注册到 Market)
```

## 示例 Agents

配置见 [`configs/agents.yaml`](configs/agents.yaml)（默认坐标围绕北京 CBD，便于「附近」演示）。

| ID | 分类 | 标签 | 端口 | 能力 |
|----|------|------|------|------|
| weather | utility | weather, forecast, 天气 | 8101 | 城市天气（演示数据） |
| localguide | lifestyle | food, travel, local, 美食 | 8102 | 本地美食/出行推荐 |
| translator | utility | language, translate, 翻译 | 8103 | 多语言翻译 |
| mediatv | entertainment | movie, tv, 电影, 电视剧… | 8104 | 爱奇艺/优酷/腾讯视频片名搜索（内置目录） |

外部 Agent（如 takeout-agent，a2a-go v2）也可向 Market 注册；Host 通过 Agent Card 自动选择协议，Bearer 用 `A2A_BEARER_TOKEN`。

## 快速开始

### 依赖

- Go 1.26+（见 `go.mod`）
- Node.js 18+（构建前端）
- 可选：OpenAI 兼容 API Key（真实模型）；无 Key 时用 `MOCK_LLM=1`

```bash
cp .env.example .env   # 或直接 export
export MOCK_LLM=1      # 离线演示（推荐先跑通）
# 使用真实模型时：
# export MOCK_LLM=0
# export OPENAI_API_KEY=sk-...
# export OPENAI_BASE_URL=https://api.openai.com/v1   # 或兼容网关
# export OPENAI_MODEL=gpt-4o-mini
# 对接 a2a-go v2 Agent 时与对方 token 一致：
# export A2A_BEARER_TOKEN=dev-a2a-secret-change-me
```

### 构建前端

```bash
cd web && npm install && npm run build && cd ..
# 或: make web
```

### 启动 Market Gateway

```bash
go run ./cmd/market
# 或: make market
# http://127.0.0.1:13100
```

未设置 `OPENAI_API_KEY` 且未开 `MOCK_LLM` 时：注册与发现仍可用；`/api/v1/chat` 不可用。

### 启动示例 Agents（另开终端）

```bash
MOCK_LLM=1 go run ./cmd/agents
# 或: make agents
```

需 `OPENAI_API_KEY` 或 `MOCK_LLM=1`。Docker / Compose 下用 `MARKET_URL`、`ADVERTISE_HOST` 指向可达地址。

## 演示路径

1. 打开 http://127.0.0.1:13100 ，允许定位（或使用默认北京坐标）。
2. 搜索「天气」「电影」等，或点「语音」；列表按距离排序。
3. 点「对话」直连某个 Agent（经 Market A2A 代理）。
4. 或对 Market Host 说：「帮我找附近的天气 Agent，问问今天天气」——Host 会调用 `discover_agents` 与 `talk_to_agent`。影视可说：「帮我找电影 Agent，搜一下狂飙」。

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/agents/register` | Agent 注册 / 心跳 |
| DELETE | `/api/v1/agents/{id}` | 注销 |
| GET | `/api/v1/agents` | 列表（默认仅 online） |
| GET | `/api/v1/agents/{id}` | 单条记录 |
| GET | `/api/v1/agents/discover?lat=&lng=&radius_km=&category=&tags=&q=` | 多维发现 |
| GET | `/api/v1/agents/{id}/card` | Agent Card（缓存或回源） |
| POST | `/api/v1/chat` | 人 → Market Host（SSE） |
| POST | `/api/v1/a2a/{id}/message:send` | 人 → 指定 Agent（A2A 代理） |
| GET | `/api/v1/geo/reverse?lat=&lng=` | 坐标反查地名 |
| GET | `/api/v1/health` | 健康检查 |

## 容器化部署

单镜像双进程：`market`（Gateway + 前端）与 `agents`（四个 A2A 示例）。Compose 网络内 Agents 以服务名 `agents` 注册 A2A 地址，Market 经 `http://agents:810x` 回调。默认 `MOCK_LLM=1`（容器内常无法直连 OpenAI）。

```bash
cp .env.example .env   # 可选填入 OPENAI_API_KEY；设 MOCK_LLM=0 用真实模型
docker compose up -d --build
# UI: http://127.0.0.1:13100
docker compose logs -f
docker compose down
```

| 服务 | 对外端口 | 说明 |
|------|----------|------|
| market | 13100 | API + 静态 UI，SQLite 卷 `market-data` |
| agents | 无（仅内部 8101–8104） | 向 `http://market:13100` 注册 |

环境变量：`OPENAI_API_KEY`、`OPENAI_BASE_URL`、`OPENAI_MODEL`、`MOCK_LLM`、`A2A_BEARER_TOKEN`；agents 另支持 `MARKET_URL`、`ADVERTISE_HOST`。

```bash
make docker-up    # 等同 docker compose up -d --build
make docker-down
```

## 开发

```bash
make test
make tidy

# API 冒烟（需先启动 market）
curl -s 'http://127.0.0.1:13100/api/v1/agents/discover?lat=39.9042&lng=116.4074&radius_km=50' | jq
curl -s 'http://127.0.0.1:13100/api/v1/health'

# 前端热更新（代理到 :13100）
cd web && npm run dev
# http://127.0.0.1:7100
```

### 目录结构

```
cmd/market          Market Gateway 入口
cmd/agents          四个示例 Agent 进程入口
agents/*            各领域 Agent 实现（Eino ChatModelAgent + tools）
configs/agents.yaml 示例 Agent 元数据与坐标
internal/
  api/              HTTP Gateway
  registry/         SQLite 注册表
  discovery/        多维发现
  hostagent/        Market Host（discover / talk 工具）
  a2a/              A2A client（legacy + a2a-go v2）
  agentboot/        Hertz + A2A 服务启动与自注册
  llm/              OpenAI 兼容 ChatModel / MOCK_LLM
  mediasearch/      影视演示目录搜索
  geo/              Haversine 与地名反查
web/                React + Vite UI
```

## 实现要点

- **Registry**：SQLite（`data/market.db`）
- **Discovery**：分类精确匹配 ∩ 标签任一命中 ∩ 关键词 ∩ 半径，按 `distance_km` 升序
- **A2A**：示例 Agent 用 Hertz + `eino-ext/a2a`；Market 侧先按 Agent Card 判断协议，失败时回退/升级到 a2a-go v2
- **LLM**：`MOCK_LLM=1` 走本地 mock（可编排工具）；否则 `OPENAI_*` OpenAI 兼容接口
- **非目标**：鉴权、多租户、向量检索、服务端 ASR、生产级 SLA
