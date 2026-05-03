MCP 的核心是使用 JSON-RPC 2.0 作为消息格式，为客户端和服务器之间的通信提供了一种标准化的方式。

基础协议定义了三种基本消息类型，分别是请求（Requests）、响应（Responses）和通知（Notifications）。

以下是这三种消息类型的详细说明：

1. 请求
请求消息用于从客户端向服务器发起操作，或者从服务器向客户端发起操作。
请求消息的结构如下：

{
  "jsonrpc": "2.0",
  "id": "string | number",
  "method": "string",
  "params": {
    "[key: string]": "unknown"
  }
}
jsonrpc：协议版本，固定为"2.0"。
id：请求的唯一标识符，可以是字符串或数字。
method：要调用的方法名称，是一个字符串。
params：方法的参数，是一个可选的键值对对象，其中键是字符串，值可以是任意类型。

2. 响应
响应消息是对请求的答复，从服务器发送到客户端，或者从客户端发送到服务器。
响应消息的结构如下：
{
  "jsonrpc": "2.0",
  "id": "string | number",
  "result": {
    "[key: string]": "unknown"
  },
  "error": {
    "code": "number",
    "message": "string",
    "data": "unknown"
  }
}

jsonrpc：协议版本，固定为"2.0"。
id：与请求中的id相对应，用于标识响应所对应的请求。
result：如果请求成功，result字段包含操作的结果，是一个键值对对象。
error：如果请求失败，error字段包含错误信息，其中：
    code：错误代码，是一个数字。
    message：错误描述，是一个字符串。
    data：可选的附加错误信息，可以是任意类型。

3. 通知
通知消息是一种单向消息，不需要接收方回复。
通知消息的结构如下：

{
  "jsonrpc": "2.0",
  "method": "string",
  "params": {
    "[key: string]": "unknown"
  }
}

jsonrpc：协议版本，固定为"2.0"。
method：要调用的方法名称，是一个字符串。
params：方法的参数，是一个可选的键值对对象，其中键是字符串，值可以是任意类型。

