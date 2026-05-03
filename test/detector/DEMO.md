## 📄 文档内容获取演示

### 📋 搜索到的文档信息

| 字段 | 内容 |
|------|------|
| 标题 | Xinjianwendang 1 |
| 类型 | DOCX |
| 创建时间 | 2026-05-03T00:38:48+08:00 |
| URL | https://jcneyh7qlo8i.feishu.cn/wiki/GLXCwtMeKiFzV7k1XyccgWgBnad |
| doc_id | GLXCwtMeKiFzV7k1XyccgWgBnad |

### 📝 文档内容（Markdown）

```markdown
Dwdadhkawhdgaku

Dawhkudhad

Dkauwhdgka
```

### 📊 文档元数据

| 字段 | 值 |
|------|-----|
| 总长度 | 39 字符 |
| log_id | 20260503005038C2EDC93C194BBC60723C |

---

### 🛠️ 使用的 lark-cli 命令

```bash
# 步骤1: 按创建时间搜索文档
lark-cli docs +search \
  --filter '{"create_time":{"start":"2026-05-03","end":"2026-05-04"},"sort_type":"CREATE_TIME"}' \
  --format pretty

# 步骤2: 获取文档内容
lark-cli docs +fetch \
  --doc "https://jcneyh7qlo8i.feishu.cn/wiki/GLXCwtMeKiFzV7k1XyccgWgBnad"
```

### ✅ 文档内容变化检测流程

1. **搜索阶段**：使用 `docs +search` + `--filter` 按时间范围搜索
2. **获取阶段**：使用 `docs +fetch` 获取文档内容
3. **提取阶段**：提取 `markdown` 字段或其他字段
4. **展示阶段**：解析并展示文档内容
