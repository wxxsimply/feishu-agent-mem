
```golang
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "github.com/invopop/jsonschema" // required go1.18+
    "github.com/volcengine/volcengine-go-sdk/service/arkruntime"
    "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
    "github.com/volcengine/volcengine-go-sdk/volcengine"
)

// 定义分步解析模型（对应业务场景的结构化响应）
type Step struct {
    Explanation string `json:"explanation" jsonschema_description:"步骤说明"`
    Output      string `json:"output" jsonschema_description:"步骤计算结果"`
}

// 定义最终响应模型（包含分步过程和最终答案）
type MathResponse struct {
    Steps       []Step `json:"steps" jsonschema_description:"解题步骤列表"`
    FinalAnswer string `json:"final_answer" jsonschema_description:"最终答案"`
}

// 复用原有 Schema 生成函数（已优化返回类型）
func GenerateSchema[T any]() *jsonschema.Schema { // <-- 优化返回类型为具体 Schema 类型
    reflector := jsonschema.Reflector{
        AllowAdditionalProperties: false,
        DoNotReference:            true,
    }
    return reflector.Reflect(new(T)) // 使用 new(T) 避免空值问题
}

// 生成数学响应的 JSON Schema
var MathResponseSchema = GenerateSchema[MathResponse]()

func main() {
    client := arkruntime.NewClientWithApiKey(
        os.Getenv("ARK_API_KEY"),
        arkruntime.WithBaseUrl("https://ark.cn-beijing.volces.com/api/v3"),
        )
    ctx := context.Background()

    // 构造请求消息（包含 system 和 user 角色）
    messages := []*model.ChatCompletionMessage{
        {
            Role: model.ChatMessageRoleSystem,
            Content: &model.ChatCompletionMessageContent{
                StringValue: volcengine.String("你是一位数学辅导老师，需详细展示解题步骤"),
            },
        },
        {
            Role: model.ChatMessageRoleUser,
            Content: &model.ChatCompletionMessageContent{
                StringValue: volcengine.String("用中文解方程组：8x + 9 = 32 和 x + y = 1"),
            },
        },
    }

    // 配置响应格式（使用 MathResponse 的 Schema）
    schemaParam := model.ResponseFormatJSONSchemaJSONSchemaParam{
        Name:        "math_response", // 对应 Python 中的响应名称
        Description: "数学题解答的结构化响应",
        Schema:      MathResponseSchema,
        Strict:      true,
    }

    // 构造请求（包含 thinking 配置）
    req := model.CreateChatCompletionRequest{
        Model:    "doubao-seed-1-6-251015", // 需替换为实际可用模型
        Messages: messages,
        ResponseFormat: &model.ResponseFormat{
            Type:       model.ResponseFormatJSONSchema,
            JSONSchema: &schemaParam,
        },
        Thinking: &model.Thinking{
            // Type: model.ThinkingTypeDisabled, // 关闭深度思考能力
            Type: model.ThinkingTypeEnabled, //开启深度思考能力
        },
    }


    // 调用 API
    resp, err := client.CreateChatCompletion(ctx, req)
    if err != nil {
        fmt.Printf("structured output chat error: %v\\n", err)
        return
    }


    // 解析结构化响应（关键差异：Go 需要手动反序列化）
    var mathResp MathResponse
    err = json.Unmarshal([]byte(*resp.Choices[0].Message.Content.StringValue), &mathResp)
    if err != nil {
        panic(err.Error())
    }


    // 打印格式化结果（使用 json.MarshalIndent 实现缩进）
    prettyJSON, _ := json.MarshalIndent(mathResp, "", "  ")
    fmt.Println(string(prettyJSON))
}
```