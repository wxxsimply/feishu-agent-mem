# OpenClaw 飞书接入配置指南

本指南说明如何将 feishu-agent-mem 记忆系统接入 OpenClaw，并配置飞书 channel。

## 一、前置条件

1. 确保 feishu-agent-mem 服务正常运行
2. 确保已安装 lark-cli 并完成登录
3. 确保 OpenClaw (openclaw-zh) 已安装

## 二、配置步骤

### 步骤 1: 启动记忆系统服务

首先确保 feishu-agent-mem 服务正在运行：

```bash
# 方式一：本地运行
make build
make run

# 方式二：Docker 运行
make docker-build
make docker-run
```

服务会在 `http://localhost:37777` 启动 MCP 服务器。

### 步骤 2: 配置 OpenClaw

创建 OpenClaw 配置文件：

```bash
# 创建 OpenClaw 配置目录
mkdir -p ~/.openclaw

# 复制示例配置
cp config/openclaw.json.example ~/.openclaw/openclaw.json
```

编辑 `~/.openclaw/openclaw.json`：

```json
{
  "mcpServers": {
    "feishu-memory": {
      "command": "none",
      "env": {},
      "url": "http://localhost:37777"
    }
  },
  "skills": {
    "feishu-memory-interface": {
      "enabled": true,
      "config": {
        "memoryServiceUrl": "http://localhost:37777",
        "autoSearchOnMessage": true,
        "maxSearchResults": 5
      }
    }
  },
  "project": {
    "name": "feishu-mem",
    "defaultTopic": "general"
  }
}
```

### 步骤 3: 配置飞书 Lark CLI

确保 lark-cli 已正确配置：

```bash
# 检查 lark-cli 配置
lark-cli config list

# 如果未配置，先登录
lark-cli auth login
# 按照提示完成飞书应用授权

# 配置默认身份为 user
lark-cli config set default_identity user
```

### 步骤 4: 将 OpenClaw 接入飞书 Channel

有两种方式将 OpenClaw 接入飞书：

#### 方式 A: 使用 OpenClaw 自带的飞书集成

如果您使用的是 openclaw-zh Docker 镜像：

```bash
# 运行 openclaw-zh 容器，挂载配置
docker run -d \
  --name openclaw-zh \
  -v ~/.openclaw:/root/.openclaw \
  -v ~/.lark-cli:/root/.lark-cli \
  -p 37778:37778 \
  openclaw-zh:latest

# 进入容器配置
docker exec -it openclaw-zh bash

# 在容器内配置飞书接入
openclaw config feishu \
  --app-id "${LARK_APP_ID}" \
  --app-secret "${LARK_APP_SECRET}" \
  --chat-id "${LARK_CHAT_IDS}"
```

#### 方式 B: 使用 lark-cli 发送消息到飞书

通过 lark-cli 将 OpenClaw 的响应发送到飞书群：

```bash
# 测试发送消息
lark-cli im +messages-send \
  --chat-id "oc_28cf78aa701166c04ad4425c53c6c225" \
  --msg-type text \
  --content "OpenClaw 记忆系统已连接！"

# 设置 OpenClaw 的输出通过 lark-cli 转发
```

### 步骤 5: 更新环境变量配置

确保 `.env` 文件包含正确的飞书配置：

```env
# 飞书应用凭证
LARK_APP_ID=cli_a979cd80c3389cc6
LARK_APP_SECRET=by0cr4N6gcurdQxf7lQzcfIto2OYjApp
LARK_CHAT_IDS=oc_28cf78aa701166c04ad4425c53c6c225

# MCP 配置
MCP_PORT=37777

# 其他配置...
```

## 三、Docker openclaw-zh 飞书插件问题诊断

如果 Docker 中的飞书插件不可用，按以下步骤检查：

### 问题 1: 检查容器内的 lark-cli 配置

```bash
# 进入容器
docker exec -it openclaw-zh bash

# 检查 lark-cli 是否安装
which lark-cli

# 检查配置文件是否存在
ls -la ~/.lark-cli/

# 检查登录状态
lark-cli auth status
```

### 问题 2: 重新配置 lark-cli

如果配置有问题，在容器内重新配置：

```bash
# 复制宿主机的 lark-cli 配置到容器
docker cp ~/.lark-cli openclaw-zh:/root/.lark-cli

# 或者在容器内重新登录
lark-cli auth login
```

### 问题 3: 检查网络连接

```bash
# 检查是否能访问飞书 API
curl -v https://open.feishu.cn

# 检查是否能访问记忆系统 MCP 服务
curl http://host.docker.internal:37777/health
```

### 问题 4: Docker 网络配置

确保两个容器在同一网络或能互相访问：

```bash
# 创建共享网络
docker network create openclaw-network

# 启动记忆系统容器连接到该网络
docker network connect openclaw-network feishu-agent-mem

# 启动 openclaw-zh 容器连接到该网络
docker network connect openclaw-network openclaw-zh
```

## 四、验证配置

### 1. 验证 MCP 服务

```bash
curl -X POST http://localhost:37777/health
# 应该返回: {"status":"ok","ts":...}
```

### 2. 验证飞书消息发送

```bash
# 使用 lark-cli 发送测试消息
lark-cli im +messages-send \
  --chat-id "oc_28cf78aa701166c04ad4425c53c6c225" \
  --msg-type text \
  --content "🎉 OpenClaw 配置测试成功！"
```

### 3. 验证 OpenClaw 集成

在 OpenClaw 中测试记忆系统：

```
# 在 OpenClaw 中询问
"搜索与项目相关的决策"

# 应该能调用 memory.search 工具并返回结果
```

## 五、常见问题排查

### Q1: MCP 连接失败

检查：
- 记忆系统服务是否启动：`curl http://localhost:37777/health`
- 端口 37777 是否被占用：`lsof -i :37777`
- 防火墙设置

### Q2: 飞书消息发送失败

检查：
- lark-cli 是否已登录：`lark-cli auth status`
- 应用权限是否足够：在飞书开放平台检查应用权限
- chat_id 是否正确

### Q3: Docker 容器间无法通信

解决：
- 使用 host 网络模式或创建自定义 bridge 网络
- 使用 `host.docker.internal` 访问宿主机服务
- 检查容器日志：`docker logs openclaw-zh`

## 六、完整的 docker-compose 配置

创建 `docker-compose.yml` 一键部署：

```yaml
version: '3.8'

services:
  feishu-mem:
    build: .
    container_name: feishu-agent-mem
    ports:
      - "37777:37777"
    volumes:
      - ./data:/opt/feishu-agent-mem/data
      - ./config:/opt/feishu-agent-mem/config
      - ./.env:/opt/feishu-agent-mem/.env
    environment:
      - CONFIG_PATH=/opt/feishu-agent-mem/config/openclaw.yaml
    networks:
      - openclaw-network
    restart: unless-stopped

  openclaw-zh:
    image: openclaw-zh:latest
    container_name: openclaw-zh
    depends_on:
      - feishu-mem
    ports:
      - "37778:37778"
    volumes:
      - ~/.openclaw:/root/.openclaw
      - ~/.lark-cli:/root/.lark-cli
    environment:
      - MEMORY_SERVICE_URL=http://feishu-mem:37777
    networks:
      - openclaw-network
    restart: unless-stopped

networks:
  openclaw-network:
    driver: bridge
```

启动：
```bash
docker-compose up -d
```

## 七、获取帮助

如遇问题，检查日志：
- 记忆系统日志：`docker logs feishu-agent-mem`
- OpenClaw 日志：`docker logs openclaw-zh`
