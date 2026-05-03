# 知识库模块 (lark-wiki) 运作计划

## 1. 模块概述

lark-wiki 用于构建项目的知识库，随着项目进展不断演进。知识库是项目文档的长期存储和组织形式。

## 2. 知识库创建

### 2.1 创建知识空间

在 Phase 0 时创建知识空间：

```bash
lark-cli wiki spaces create --name "YatSenOS 项目知识库" --description "YatSenOS 操作系统项目的知识库，包含架构设计、技术文档、会议纪要等" --as user
```

### 2.2 获取 space_id

```bash
lark-cli wiki spaces list --as user
```

## 3. 知识库结构演变

### 3.1 V1 结构 (Phase 0-1)

创建时间：Phase 0 第 2 天

```
YatSenOS Wiki
├── 项目概述
│   ├── 项目目标与范围
│   ├── 团队成员与分工
│   └── 项目时间线
├── 开发环境指南
│   ├── WSL2 配置
│   ├── Ubuntu 配置
│   ├── Rust 安装
│   └── QEMU 配置
└── 代码规范
    ├── 命名规范
    ├── 提交规范
    └── 分支策略
```

**创建命令**:
```bash
# 创建根节点
lark-cli wiki +node-create --space-id $SPACE_ID --title "YatSenOS Wiki" --as user

# 创建子节点
lark-cli wiki +node-create --space-id $SPACE_ID --parent $ROOT_TOKEN --title "项目概述" --as user
lark-cli wiki +node-create --space-id $SPACE_ID --parent $ROOT_TOKEN --title "开发环境指南" --as user
lark-cli wiki +node-create --space-id $SPACE_ID --parent $ROOT_TOKEN --title "代码规范" --as user
```

### 3.2 V2 结构 (Phase 2-3)

更新时间：Phase 2 第 1 天

```
YatSenOS Wiki
├── 项目概述
│   ├── 项目目标与范围
│   ├── 团队成员与分工
│   └── 项目时间线
├── 开发环境指南
│   ├── WSL2 配置
│   ├── Ubuntu 配置
│   ├── Rust 安装
│   └── QEMU 配置
├── 代码规范
│   ├── 命名规范
│   ├── 提交规范
│   └── 分支策略
├── 架构设计 (新增)
│   ├── 引导流程
│   │   ├── UEFI 启动过程
│   │   ├── ELF 加载
│   │   └── 页表映射
│   ├── 中断处理
│   │   ├── IDT 注册
│   │   ├── APIC 初始化
│   │   └── 时钟中断
│   └── 进程模型
│       ├── PCB 设计
│       ├── 上下文切换
│       └── 调度器
└── 技术决策记录 (新增)
    ├── ADR-001: 开发环境选择
    ├── ADR-002: 编译目标配置
    └── ADR-003: 进程模型设计
```

**新增节点命令**:
```bash
lark-cli wiki +node-create --space-id $SPACE_ID --parent $ROOT_TOKEN --title "架构设计" --as user
lark-cli wiki +node-create --space-id $SPACE_ID --parent $ROOT_TOKEN --title "技术决策记录" --as user
```

### 3.3 V3 结构 (Phase 4-5)

更新时间：Phase 4 第 1 天

```
YatSenOS Wiki
├── 项目概述
│   ├── 项目目标与范围
│   ├── 团队成员与分工
│   └── 项目时间线
├── 开发环境指南
│   ├── WSL2 配置
│   ├── Ubuntu 配置
│   ├── Rust 安装
│   └── QEMU 配置
├── 代码规范
│   ├── 命名规范
│   ├── 提交规范
│   └── 分支策略
├── 架构设计
│   ├── 引导流程
│   ├── 中断处理
│   ├── 进程模型
│   ├── 用户态运行时 (新增)
│   │   ├── 系统调用
│   │   ├── 用户态库
│   │   └── Shell
│   └── 并发机制 (新增)
│       ├── fork 实现
│       ├── 自旋锁
│       └── 信号量
├── 技术决策记录
│   ├── ADR-001: 开发环境选择
│   ├── ADR-002: 编译目标配置
│   ├── ADR-003: 进程模型设计
│   ├── ADR-004: 系统调用设计 (新增)
│   └── ADR-005: fork 语义定义 (新增)
├── API 文档 (新增)
│   ├── 系统调用接口
│   ├── 内核模块接口
│   └── 用户态库接口
└── 会议纪要归档 (新增)
    ├── Phase 0 会议纪要
    ├── Phase 1 会议纪要
    └── Phase 2 会议纪要
```

### 3.4 V4 结构 (Phase 6-7)

更新时间：Phase 6 第 1 天

```
YatSenOS Wiki
├── 项目概述
│   ├── 项目目标与范围
│   ├── 团队成员与分工
│   ├── 项目时间线
│   └── 项目回顾与总结 (新增，Phase 8 时补充)
├── 开发环境指南
├── 架构设计
│   ├── 系统总览 (新增架构图)
│   ├── 引导流程
│   ├── 中断处理
│   ├── 进程模型
│   ├── 用户态运行时
│   ├── 并发机制
│   ├── 存储子系统 (新增)
│   │   ├── ATA 驱动
│   │   ├── MBR 分区表
│   │   └── FAT16 文件系统
│   └── 内存管理 (新增)
│       ├── 帧分配器
│       ├── 堆内存管理
│       ├── 栈增长
│       └── brk 系统调用
├── 技术决策记录
│   ├── ... (之前的 ADR)
│   ├── ADR-006: 文件系统选择 (新增)
│   └── ADR-007: 内存回收策略 (新增)
├── API 文档
├── 测试报告 (新增)
│   ├── Phase 5 测试报告
│   └── Phase 6 测试报告
└── 会议纪要归档
```

### 3.5 V5 结构 (Phase 8)

更新时间：Phase 8 结束

```
YatSenOS Wiki
├── 项目概述
│   ├── 项目目标与范围
│   ├── 团队成员与分工
│   ├── 项目时间线
│   └── 项目回顾与总结
├── 开发环境指南
├── 架构设计
│   ├── 系统总览
│   ├── 各子系统文档
│   └── 扩展特性文档 (新增)
│       ├── tmpfs 实现
│       └── 多核调度
├── 技术决策记录
├── API 文档
├── 测试报告
├── 会议纪要归档
└── 经验总结 (新增)
    ├── 技术经验
    ├── 项目管理经验
    └── 团队协作经验
```

## 4. 知识库更新时机

### 4.1 阶段开始时

- 新增架构设计章节
- 新增技术决策记录

### 4.2 里程碑达成时

- 更新架构图
- 补充 API 文档

### 4.3 阶段结束时

- 更新测试报告
- 归档会议纪要
- 更新阶段总结

### 4.4 冲突解决后

- 记录技术决策（ADR）

## 5. 使用 lark-cli 操作知识库

### 5.1 创建节点

```bash
lark-cli wiki +node-create --space-id $SPACE_ID --parent $PARENT_TOKEN --title "节点标题" --as user
```

### 5.2 移动节点

```bash
lark-cli wiki move --node-token $NODE_TOKEN --parent $NEW_PARENT_TOKEN --as user
```

### 5.3 删除节点

```bash
lark-cli wiki delete --node-token $NODE_TOKEN --as user
```

### 5.4 获取节点信息

```bash
lark-cli wiki spaces get_node --params '{"token":"$NODE_TOKEN"}' --as user
```

## 6. 知识库与群聊的联动

### 6.1 结构变更通知

每次知识库结构调整后，在群聊中通知：
```
P7 张若琳: @所有人 知识库更新了！
P7 张若琳: 新增了「架构设计」下的「用户态运行时」章节
P7 张若琳: 大家可以把自己负责的模块文档补充进去
```

### 6.2 文档补充讨论

在群聊中讨论知识库内容：
```
P2 林晓薇: @P7 中断处理的文档我写好了，帮我放到知识库里
P7 张若琳: 好的，我放到「架构设计 > 中断处理」下面
P7 张若琳: 放好了，你看看位置对不对
```

### 6.3 结构调整讨论

在群聊中讨论知识库结构：
```
P7 张若琳: 我觉得「代码规范」可以合并到「项目概述」里，大家觉得呢？
P1 陈卓远: 可以，这样更简洁
P7 张若琳: 好的，我调整一下
```
