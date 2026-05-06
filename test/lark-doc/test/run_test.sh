#!/bin/bash
# lark-doc 决策提取端到端测试（自然文档风格）
set -e

DOC_TOKEN="CKCtduQ03o7mSTx7KdJcGZJqnrb"
PROJECT_DIR="/Users/halllo/openclaw-workspace/feishu-agent-mem"
LOG_FILE="$PROJECT_DIR/logs/test_$(date +%Y%m%d_%H%M%S).log"

mkdir -p "$PROJECT_DIR/logs"

log() { echo "[$(date +%H:%M:%S)] $1" | tee -a "$LOG_FILE"; }

append_doc() {
    local content="$1"
    local label="$2"
    local num="$3"
    local total="$4"

    log "[$num/$total] $label"
    output=$(lark-cli docs +update --doc "$DOC_TOKEN" --mode append --markdown "$content" 2>&1)
    if echo "$output" | grep -q '"ok": true'; then
        log "  ✅ OK"
    else
        log "  ⚠️  $(echo "$output" | head -2)"
    fi
    sleep 2
}

add_comment() {
    local content="$1"
    local label="$2"
    local num="$3"
    local total="$4"

    log "[$num/$total] 评论: $label"
    # 转义 content 中的双引号
    local escaped=$(echo "$content" | sed 's/"/\\"/g' | sed ':a;N;$!ba;s/\n/\\n/g')
    output=$(lark-cli drive file.comments create_v2 \
        --params "{\"file_token\": \"$DOC_TOKEN\"}" \
        --data "{\"file_type\": \"docx\", \"reply_elements\": [{\"type\": \"text\", \"text\": \"$escaped\"}]}" 2>&1)
    if echo "$output" | grep -q '"code": 0\|"ok": true'; then
        log "  ✅ OK"
    else
        log "  ⚠️  $(echo "$output" | head -2)"
    fi
    sleep 2
}

echo "========================================" | tee "$LOG_FILE"
echo "  lark-doc 决策提取测试 (自然文档风格)" | tee -a "$LOG_FILE"
echo "  $(date)" | tee -a "$LOG_FILE"
echo "  文档: $DOC_TOKEN" | tee -a "$LOG_FILE"
echo "========================================" | tee -a "$LOG_FILE"

# ===== 步骤一：停旧进程、编译 =====
log "步骤一：停旧进程、编译..."
docker exec openclaw-zh bash -c 'pkill -9 mem-service 2>/dev/null; echo done'
sleep 1
docker exec openclaw-zh bash -c 'cd /root/openclaw-workspace/feishu-agent-mem && export PATH=/usr/local/go/bin:$PATH && go build -o bin/mem-service ./cmd/mem-service/main.go'
log "编译完成"

# ===== 步骤二：启动 mem-service =====
log "步骤二：启动 mem-service..."
docker exec openclaw-zh bash -c 'cd /root/openclaw-workspace/feishu-agent-mem && ./bin/mem-service >> logs/app.log 2>&1 &'
sleep 3
log "mem-service 已启动"

# ===== 步骤三：执行 50 条修改 =====
log "步骤三：开始执行文档修改"
echo "" | tee -a "$LOG_FILE"

TOTAL=50
NUM=0

# ── 新增决策（自然嵌入） ──────────────────────────

NUM=$((NUM+1))
append_doc '
## 2.4 日志收集

团队讨论后决定引入 EFK（Elasticsearch + Fluentd + Kibana）栈做集中日志收集。之前用的 Loki 在高并发场景下查询延迟太高，换成 Elasticsearch 后虽然存储成本增加，但查询体验好了很多。

Fluentd 作为日志采集 agent，以 DaemonSet 方式部署在每个节点上。Kibana 做可视化看板。

负责人：刘洋
生效时间：5月15日
' "2.4 日志收集方案确认" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 2.5 容器编排

经过基础设施团队评估，决定采用 Kubernetes 管理容器编排。虽然当前规模用 Docker Compose 也能跑，但考虑到下半年业务量翻倍的预期，提前上 K8s 可以避免后续迁移的痛苦。

初步规划：3 master + 5 worker，使用 kubeadm 部署。

负责人：郑涛
生效时间：6月1日
' "2.5 容器编排方案" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 5.3 告警通道

除了飞书群通知外，增加短信告警通道。P0 级别故障同时走飞书 + 短信，P1 只走飞书。短信通道使用阿里云 SMS 服务。

负责人：郑涛
' "5.3 告警通道扩展" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 6.3 数据加密

安全评审结论：所有敏感数据（用户手机号、身份证号）在存储时使用 AES-256-GCM 加密，传输层走 TLS 1.3。密钥管理使用 Vault，定期轮换。

负责人：吴刚
生效时间：立即
' "6.3 数据加密方案" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 3.4 网络架构

服务间通信采用 mTLS，通过 Istio service mesh 管理。外部流量入口使用 Nginx Ingress Controller。

前期先在 staging 环境验证，确认稳定后再推到 production。

负责人：赵强
' "3.4 网络架构方案" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 2.6 API 网关

经过讨论，决定在所有微服务前面加一层 API 网关。选型上对比了 Kong、APISIX 和自研方案。

最终选择 APISIX，主要因为：
- 基于 Nginx + Lua，性能好
- 插件生态丰富
- 支持动态路由，不需要重启

负责人：孙伟
生效时间：5月底
' "2.6 API 网关选型" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 4.3 测试策略

单元测试覆盖率达到 70% 以上，集成测试覆盖核心链路。测试框架统一使用 Go 标准 testing + testify。

覆盖率纳入 CI 门禁，低于 70% 不允许合并。

负责人：周敏
' "4.3 测试策略" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 6.4 认证方案细节

飞书 OAuth 登录流程确认：
1. 用户点击"飞书登录"跳转到飞书授权页
2. 授权回调拿到 access_token
3. 后端用 access_token 换取用户信息
4. 签发自有 JWT，有效期 2 小时
5. Refresh Token 有效期 7 天

第三方 API 调用方使用 HMAC 签名认证。

负责人：吴刚
' "6.4 认证流程确认" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 7.2 人员分工

- 莫文豪：整体架构设计 + LLM 提取模块
- 张雨婷：数据库方案 + 存储层
- 李明：消息队列 + 异步任务
- 赵强：CI/CD + 部署流程
- 刘洋：日志 + 监控
- 郑涛：基础设施 + K8s
- 吴刚：安全 + 认证
- 孙伟：API 网关
- 周敏：测试
- 陈雪：前端管理后台
' "7.2 人员分工表" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 2.7 服务注册与发现

决定使用 Consul 作为服务注册与发现中心。各服务启动时向 Consul 注册，健康检查通过 HTTP 探针方式。

Consul 集群部署 3 节点，使用 Raft 协议保证一致性。

负责人：赵强
' "2.7 服务注册与发现" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 3.5 数据库连接池配置

MySQL 连接池参数最终确认：
- 最大连接数：100（之前设的 200 太大，实际用不到）
- 最小空闲连接：10
- 连接超时：5s
- 空闲超时：300s
- 慢查询阈值：200ms

负责人：张雨婷
' "3.5 连接池配置" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 5.4 链路追踪

接入 OpenTelemetry 做分布式链路追踪，Tracer 数据导出到 Jaeger。

每个请求生成唯一的 trace_id，贯穿网关 → 服务 → 数据库全链路。采样率设为 10%，生产环境可动态调整。

负责人：刘洋
生效时间：6月
' "5.4 链路追踪方案" "$NUM" "$TOTAL"

# ── 重复决策（措辞微调） ──────────────────────────

NUM=$((NUM+1))
append_doc '
## 补充确认：缓存方案

再次确认 Redis 作为缓存层的方案。当前 2GB 内存配置先用着，等月底压测结果出来后再决定是否扩容。

负责人：王磊
' "重复：缓存方案确认" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充确认：CI 工具

GitHub Actions 方案确认没问题，已经在 staging 环境跑通了完整流水线。下周推到 production。

负责人：赵强
' "重复：CI 工具确认" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充确认：数据库

MySQL 8.0 方案不变。张雨婷已经完成了 schema 设计，本周五前提交评审。

负责人：张雨婷
' "重复：数据库确认" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充确认：日志收集

EFK 方案确认。Fluentd 的 Docker 镜像已经构建完成，待 K8s 集群就绪后部署。

负责人：刘洋
' "重复：日志收集确认" "$NUM" "$TOTAL"

# ── 冲突决策 ──────────────────────────────────────

NUM=$((NUM+1))
append_doc '
## 3.6 数据库方案调整

最近重新评估后，觉得 PostgreSQL 在 JSONB 和全文搜索方面确实比 MySQL 强不少。建议改用 PostgreSQL 作为主数据库，MySQL 只保留用于兼容旧系统。

需要张雨婷评估迁移成本。

负责人：莫文豪
' "冲突：MySQL → PostgreSQL" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 2.8 缓存内存调整

压测结果显示 2GB Redis 在高峰期频繁触发淘汰策略，建议直接扩到 8GB。同时启用 Redis Cluster 模式做水平扩展。

负责人：王磊
生效时间：立即
' "冲突：Redis 2GB → 8GB" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 4.4 CI 工具变更

DevOps 团队反馈 GitHub Actions 在构建大型 Docker 镜像时速度很慢，建议改用 Jenkins，内网构建速度更快。

Jenkins 已经在内网有一台闲置服务器，可以直接复用。

负责人：赵强
' "冲突：GitHub Actions → Jenkins" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 6.5 权限模型调整

安全团队建议把 RBAC 改成 ABAC。理由是我们的场景有大量动态权限需求（比如"只能看自己负责的模块"），RBAC 的角色定义会变得很臃肿。

ABAC 基于属性判断，更灵活。但实现复杂度确实高不少。

负责人：吴刚
生效时间：6月中旬
' "冲突：RBAC → ABAC" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 2.9 日志框架变更

slog 在压测中表现不如预期，部分场景下比 Zap 慢了 15%。建议还是用 Zap，虽然多一个依赖，但性能更稳定。

负责人：刘洋
' "冲突：slog → Zap" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 4.5 测试框架调整

testify 的 mock 模块用起来不太顺手，建议换成 gomock。gomock 可以从 interface 自动生成 mock 代码，减少手写 mock 的工作量。

负责人：周敏
' "冲突：testify → gomock" "$NUM" "$TOTAL"

# ── 回溯决策 ──────────────────────────────────────

NUM=$((NUM+1))
append_doc '
## 2.10 日志收集回退

EFK 方案在实际部署中遇到问题：Elasticsearch 占用资源太多，3 节点集群吃掉了 12GB 决策。决定回退到 Loki，虽然查询慢一点，但资源消耗小很多。

负责人：刘洋
生效时间：立即
' "回溯：EFK → Loki" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 3.7 数据库回退

PostgreSQL 的运维工具链不如 MySQL 成熟，团队学习成本比预期高。决定回退到 MySQL 8.0，不折腾了。

负责人：莫文豪、张雨婷
生效时间：立即
' "回溯：PostgreSQL → MySQL" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 2.11 API 网关回退

APISIX 的 Lua 插件开发门槛比预期高，团队没有 Lua 经验。决定暂时不上 API 网关，先用 Nginx 做简单的反向代理和限流。

后续有需要再评估。

负责人：孙伟
生效时间：立即
' "回溯：APISIX → Nginx" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 5.5 链路追踪回退

Jaeger 部署后发现和现有 Prometheus + Grafana 栈的集成不够顺畅，两套系统看数据很割裂。决定先用 Prometheus 的 Exemplar 功能关联日志和指标，不上独立的链路追踪。

负责人：刘洋
生效时间：立即
' "回溯：Jaeger → Prom Exemplar" "$NUM" "$TOTAL"

# ── 更新已有决策 ──────────────────────────────────

NUM=$((NUM+1))
append_doc '
## 更新：K8s 规格调整

由于预算限制，K8s 集群从 3 master + 5 worker 缩减为 1 master + 3 worker。单 master 有单点风险，但当前阶段可以接受，后续再扩展。

负责人：郑涛
' "更新：K8s 规格缩减" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 更新：告警阈值调整

P0 故障定义从"服务不可用"改为"核心接口错误率 > 1%"。之前的定义太宽泛，导致很多实际影响用户的故障没被及时发现。

飞书通知 + 短信双通道不变。

负责人：郑涛
' "更新：P0 定义调整" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 更新：JWT 有效期调整

Access Token 有效期从 2 小时缩短到 30 分钟。安全审计反馈 2 小时太长，token 泄露风险高。

Refresh Token 有效期保持 7 天不变。

负责人：吴刚
' "更新：JWT 有效期缩短" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 更新：测试覆盖率门槛

单元测试覆盖率门槛从 70% 上调到 80%。之前设 70% 太低了，核心模块应该有更高的覆盖。

集成测试覆盖核心链路不变。

负责人：周敏
' "更新：覆盖率门槛上调" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 更新：备份策略细化

全量备份时间从凌晨 2:00 改为 3:00，避开其他定时任务。增量备份频率从每小时改为每 30 分钟。

保留天数从 30 天增加到 45 天。

负责人：张雨婷
' "更新：备份策略细化" "$NUM" "$TOTAL"

# ── 非决策内容 ────────────────────────────────────

NUM=$((NUM+1))
append_doc '
## 会议纪要：5月6日架构评审

参会人：莫文豪、张雨婷、李明、赵强、刘洋

讨论要点：
1. 缓存层方案基本确认，细节待压测后调整
2. 日志收集方案有分歧，EFK vs Loki 需要再评估
3. CI/CD 流水线已在 staging 跑通
4. 数据库读写分离方案需要补充 failover 机制

下次会议：5月13日，确认 Phase 1 具体排期
' "会议纪要" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 项目进度更新（截至5月6日）

- 缓存层：Redis 已部署完成，接入代码编写中（60%）
- 日志：EFK 方案验证中，遇到资源问题
- CI/CD：GitHub Actions 流水线已跑通，等待 production 环境审批
- 数据库：Schema 设计完成，评审中
- K8s：集群规划中，预算待确认
- 安全：认证方案设计完成，开始编码
' "进度更新" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 待办事项

- 张雨婷：周五前提交数据库 Schema 评审
- 刘洋：本周五前完成 EFK vs Loki 资源对比报告
- 赵强：申请 production 环境 GitHub Actions 权限
- 郑涛：提交 K8s 集群采购申请
- 吴刚：完成认证模块单元测试
- 王磊：本周完成 Redis 压测，输出报告
' "待办" "$NUM" "$TOTAL"

# ── 更多更新和新决策 ──────────────────────────────

NUM=$((NUM+1))
append_doc '
## 8.2 风险更新

新增风险：EFK 方案的 Elasticsearch 资源消耗超出预期，可能影响其他服务。需要在本周内确定是扩容还是回退到 Loki。

已缓解风险：CI/CD 流水线已在 staging 验证通过。

负责人：莫文豪
' "风险更新" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 9. 技术债务清理计划

在升级过程中需要顺便清理以下技术债务：
1. 统一错误码定义（当前各模块各自定义，很混乱）
2. 清理废弃 API（v1 接口还有 3 个在用）
3. 依赖版本升级（Go 1.21 → 1.22）

每个模块负责人在 Phase 1 结束前完成自己模块的债务清理。

负责人：莫文豪
' "技术债务清理" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 10. 文档规范

后续所有技术方案文档统一使用以下结构：
1. 背景与目标
2. 技术选型（含对比表格）
3. 详细设计
4. 风险评估
5. 里程碑计划

文档放在飞书知识库的「技术方案」空间下。

负责人：莫文豪
' "文档规范" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充：Nginx 反向代理配置

Nginx 反向代理的基本配置已经写好，包含：
- 负载均衡（round-robin）
- 限流（每秒 1000 请求）
- 超时设置（连接 5s，读写 30s）
- HTTPS 强制跳转

配置文件放在仓库的 deploy/nginx/ 目录下。

负责人：赵强
' "Nginx 配置" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充：Prometheus 监控配置

Prometheus 已经部署完成，采集间隔 15s，数据保留 15 天。

已配置的告警规则：
- CPU 使用率 > 80% 持续 5 分钟
- 内存使用率 > 85%
- 磁盘使用率 > 90%
- 接口 P99 > 500ms
- 错误率 > 1%

负责人：刘洋
' "Prometheus 配置" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充：Vault 密钥管理

HashiCorp Vault 已部署，用于管理以下密钥：
- 数据库连接密码
- API 密钥
- JWT 签名密钥
- 加密密钥（AES-256-GCM）

自动轮换策略：数据库密码每 90 天轮换，API 密钥每 30 天轮换。

负责人：吴刚
' "Vault 配置" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 补充：数据库监控

MySQL 慢查询日志已开启，阈值 200ms。每天自动分析慢查询并生成报告。

核心监控指标：
- QPS / TPS
- 连接数
- 慢查询数量
- 主从延迟
- Buffer Pool 命中率

负责人：张雨婷
' "数据库监控" "$NUM" "$TOTAL"

NUM=$((NUM+1))
append_doc '
## 11. 后续规划

Phase 1 完成后，Phase 2 的重点是模块拆分。初步规划：
- 用户服务独立部署
- 订单服务独立部署
- 通知服务独立部署
- 网关层统一流量入口

模块间通信使用 gRPC + 消息队列。

负责人：莫文豪
' "后续规划" "$NUM" "$TOTAL"

# ── 评论（反对意见 + 补充 + 提问）──────────────────

NUM=$((NUM+1))
add_comment "EFK 回退到 Loki 的决定我有顾虑。Loki 的查询性能在数据量大了之后会更差，到时候再换 EFK 成本更高。建议还是想办法优化 ES 的资源配置。" "评论：Loki 回退顾虑" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "MySQL vs PostgreSQL 这个问题，我觉得应该做个正式的 POC 对比，而不是凭印象决策。两个数据库在 JSONB、全文搜索、并发性能上都跑一遍 benchmark 再定。" "评论：数据库需要 POC" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "Redis 从 2GB 扩到 8GB 有点激进。建议先扩到 4GB 观察一周，确认实际用量再决定。避免资源浪费。" "评论：Redis 扩容建议" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "Jenkins 内网构建确实快，但 GitHub Actions 的优势在于和代码仓库的深度集成。建议做个构建速度对比再决定。" "评论：CI 工具对比" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "ABAC 实现复杂度确实高，但我们团队目前没有 ABAC 的经验。建议先用 RBAC 覆盖 80% 的场景，剩下 20% 用代码硬编码，等团队积累了经验再迁移到 ABAC。" "评论：权限模型建议" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "K8s 单 master 风险太大了。master 挂了整个集群就瘫了。至少要 3 master 做高可用，预算不够的话可以考虑 k3s。" "评论：K8s 高可用" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "APISIX 的 Lua 门槛确实是个问题。但如果不上网关，后续流量治理会很被动。建议考虑 Kong，虽然也是 Lua 但文档和社区更成熟。" "评论：网关替代方案" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "JWT 30 分钟有效期太短了，用户每半小时就要重新登录一次，体验很差。建议 Access Token 1 小时，配合 Refresh Token 静默续期。" "评论：JWT 有效期" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "支持回退到 Loki。当前阶段日志量不大，Loki 够用。等日志量上来再考虑 EFK 也不迟。" "评论：支持 Loki 回退" "$NUM" "$TOTAL"

NUM=$((NUM+1))
add_comment "测试覆盖率 80% 的目标很好，但要注意不要为了凑覆盖率写无意义的测试。重点覆盖核心业务逻辑和边界条件。" "评论：覆盖率质量" "$NUM" "$TOTAL"

# ===== 步骤四：等待处理 =====
echo "" | tee -a "$LOG_FILE"
log "========================================"
log "所有 $TOTAL 条修改完成！"
log "========================================"
echo "" | tee -a "$LOG_FILE"

log "等待 60 秒让 mem-service 处理..."
sleep 60

log "最近日志（最后 80 行）："
echo "---" | tee -a "$LOG_FILE"
docker exec openclaw-zh bash -c 'tail -80 /root/openclaw-workspace/feishu-agent-mem/logs/app.log 2>/dev/null' | tee -a "$LOG_FILE"
echo "---" | tee -a "$LOG_FILE"

log "测试完成！日志: $LOG_FILE"
