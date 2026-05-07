## 测试流程

@lark-im测试流程.md  当前lark-doc测试完毕 参照文档 测试lark-im 在测试之前 需要修正下git存储库里文件夹命名的一个小bug
  我在decisions/feishu-mem/目录下发现有三个文件夹"general"/"general通用"/"通用" 命名混乱 需要修正

### 步骤一

先检查本地mem-service进程 如果有 关闭所有进程 为了方便测试 缩小检测器时间间隔 更新openclaw.yaml 确保时间间隔被应用 与此同时 开启防抖功能 预防修改被分割 然后重新使用`go build -o bin/mem-service ./cmd/mem-service/main.go`编译mem-service服务 确保编译通过才进行下一步测试

### 步骤二

