# 日程检测器修复

## 问题
无法检测用户新创建的日程

## 原因
原实现没有使用快照对比历史状态，只是简单地检查日程的 start_time 是否在 cutoff 之后，这导致：
1. 无法真正识别"新"创建的日程
2. 容易误报/漏报
3. 无法检测日程更新

## 解决方案
参考 `WikiExtractor` 的实现，为 `CalendarExtractor` 添加了快照功能：

### 新增数据结构
- `CalendarSnapshot` - 日程状态快照
- `EventSnapshot` - 单个日程快照

### 新增方法
- `snapshotFilePath()` - 快照文件路径
- `loadSnapshot()` - 加载历史快照
- `saveSnapshot()` - 保存当前快照
- `buildCurrentState()` - 构建当前状态

### 改进的 `Detect()` 方法
1. 获取当前日程状态（过去7天到未来30天）
2. 加载上次快照
3. 对比找出新增和更新的日程
4. 保存当前快照

### 改进的 `parseAgendaEvents()` 方法
- 尝试获取真实的 event_id
- 支持多种字段名（event_id/id, summary/title, start/start_time）
- 如果没有真实 ID，用 title+startTime 生成稳定 ID

## 工作原理
首次运行：保存基线快照，不报告变化  
后续运行：对比当前状态与上次快照，报告新增/更新的日程
