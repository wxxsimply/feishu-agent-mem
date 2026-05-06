#!/usr/bin/env python3
"""基于 Yatsenos OS 实验文档生成模拟群聊消息"""
import json
import random
import hashlib

random.seed(42)

# 21 个用户（与 webhooks.md 一致）
USERS = [
    "Zack", "Floy", "Emmett", "Tanesha", "Ji",
    "Noel", "Hobert", "Nerissa", "Kylie", "Stefan",
    "Monroe", "Tobias", "Pei", "Maryalice", "Michell",
    "Emmie", "Micki", "Rebecca", "Kendall", "Danyel", "Hai"
]

# ========== 模拟消息模板 ==========

# 1. 决策消息（项目的技术选型和决定）
DECISION_MSGS = [
    # 实验2 - 中断相关
    {"user": "Zack",    "text": "关于APIC的实现方案，我建议直接用XAPIC，不需要支持IOAPIC，代码量能少一半"},
    {"user": "Floy",    "text": "但文档说了未来要支持多核，IOAPIC是必须的吧"},
    {"user": "Zack",    "text": "那先实现XAPIC基础功能，IOAPIC留个接口后面再补"},
    {"user": "Tanesha", "text": "好，那就定下来：先做XAPIC，IOAPIC留接口，后面迭代"},
    {"user": "Zack",    "text": "决定用 `bitflags` 和 `bit_field` 来定义APIC寄存器，可读性会好很多"},
    {"user": "Floy",    "text": "可以，不过要确保寄存器偏移常量也要用enum管理"},
    {"user": "Tanesha", "text": "确认采用bitflags方案，register offset统一用enum"},
    {"user": "Emmett",  "text": "时钟中断频率定多少？文档说可以自己调"},
    {"user": "Hobert",  "text": "100Hz吧，太频繁了影响性能"},
    {"user": "Emmett",  "text": "但文档里示例是用的高频，我们要不要跟他保持一致？"},
    {"user": "Hobert",  "text": "先用100Hz，后面看测试结果再调"},
    {"user": "Michell", "text": "决定一下串口中断的缓冲区大小吧，文档推荐128"},
    {"user": "Floy",    "text": "128不够，一次可能出现大量的输入，建议256"},
    {"user": "Michell", "text": "那就256，用 `ArrayQueue` 实现"},
    {"user": "Floy",    "text": "好，通过，缓冲区256 + ArrayQueue"},
    # 实验3 - 进程相关
    {"user": "Tobias",  "text": "进程栈分配方案，文档说按PID偏移，我觉得这个设计不太好"},
    {"user": "Zack",    "text": "但是这是最简单的实现方式，后面可以再优化"},
    {"user": "Tobias",  "text": "行，先用PID偏移方案，但预留重构接口"},
    {"user": "Zack",    "text": "确认，进程栈按 `STACK_MAX - pid * 4GiB` 分配"},
    {"user": "Noel",    "text": "调度器方案讨论一下，文档给的FIFO，但我觉得需要优先级"},
    {"user": "Kendall", "text": "FIFO先实现，优先级后面再加，不然scope太大"},
    {"user": "Noel",    "text": "同意，先FIFO可运行，不阻塞主流程"},
    {"user": "Stefan",  "text": "进程页表怎么处理？我想直接clone根节点"},
    {"user": "Pei",     "text": "文档说了在内核假设下clone根节点就够了，省内存"},
    {"user": "Stefan",  "text": "OK，就用clone根节点方案，不递归复制整棵树"},
    {"user": "Hai",     "text": "缺页异常处理范围确定一下，我只想处理栈缺页"},
    {"user": "Danyel",  "text": "对，其他缺页直接panic，降低复杂度"},
    {"user": "Hai",     "text": "决定：缺页只处理栈自动扩容，其余直接panic"},
    # 架构决策
    {"user": "Kylie",   "text": "GDT的IST栈分配，Double Fault和Page Fault各需要独立的栈"},
    {"user": "Monroe",  "text": "Double Fault栈大小建议4096，Page Fault栈2048"},
    {"user": "Kylie",   "text": "各分配一个4KiB页面吧，统一大小简化管理"},
    {"user": "Monroe",  "text": "同意，统一4KiB"},
    {"user": "Ji",      "text": "思考题1，我建议直接用 panic 来响应全部异常"},
    {"user": "Nerissa", "text": "也可以，但至少异常信息要打印完整"},
    {"user": "Ji",      "text": "对，每个异常处理带 `{:#?}` 打印stack frame"},
    {"user": "Nerissa", "text": "确认，全部异常注册处理程序 + full stack dump"},
    {"user": "Micki",   "text": "我建议把COUNTER定义为 `AtomicU64`，保证线程安全"},
    {"user": "Rebecca", "text": "或者用 `Mutex<u64>` 也行"},
    {"user": "Micki",   "text": "AtomicU64更轻量，无锁操作"},
    {"user": "Rebecca", "text": "好，用AtomicU64"},
    {"user": "Emmie",   "text": "测试内核线程的创建参数，我建议至少创建3个并发线程"},
    {"user": "Maryalice", "text": "3个够吗？我想测5个"},
    {"user": "Emmie",   "text": "3个够了，能验证并发调度就行，多了反而干扰调试"},
    {"user": "Maryalice", "text": "好，3个线程"},
]

# 2. 冲突决策（反对意见、争论）
CONFLICT_MSGS = [
    {"user": "Floy",    "text": "我不赞成用XAPIC，现在都是x2APIC了，应该直接上x2APIC"},
    {"user": "Zack",    "text": "但是文档只给了XAPIC的示例代码，x2APIC要自己从头写"},
    {"user": "Floy",    "text": "x2APIC有MSR寄存器，操作更简单，而且性能更好"},
    {"user": "Tanesha", "text": "Floy说的有道理，但Zack说的也对。先按文档走XAPIC，迭代时再升级"},
    {"user": "Floy",    "text": "行，但标记个TODO，明确后面要升级"},
    {"user": "Hobert",  "text": "我觉得100Hz太慢了，Tick精度不够，建议1000Hz"},
    {"user": "Emmett",  "text": "1000Hz太频繁了吧，每秒1000次中断，光切换上下文就占不少CPU"},
    {"user": "Hobert",  "text": "如果要做进程调度，100Hz的粒度太粗了，时间片管理不精确"},
    {"user": "Emmett",  "text": "折中一下？500Hz"},
    {"user": "Hobert",  "text": "行，500Hz"},
    {"user": "Noel",    "text": "我不认同PID从1开始，应该从0开始，和数组索引一致"},
    {"user": "Zack",    "text": "但文档说了0表示无进程，PID从1开始"},
    {"user": "Noel",    "text": "文档这个设计有点奇怪，不过算了，按文档来"},
    {"user": "Pei",     "text": "进程切换时我觉得应该先切换页表再加载上下文"},
    {"user": "Stefan",  "text": "不对，应该先保存上下文再切换页表"},
    {"user": "Pei",     "text": "为什么？页表切换后虚拟地址变了，加载上下文会出问题"},
    {"user": "Stefan",  "text": "先保存上下文到当前进程，再切页表，再恢复新进程上下文"},
    {"user": "Pei",     "text": "明白了，确实这样才对"},
    {"user": "Nerissa", "text": "我认为引入ahash是过度设计，直接用标准hash就行"},
    {"user": "Ji",      "text": "ahash在no_std环境下性能更好，文档也是这么推荐的"},
    {"user": "Nerissa", "text": "有benchmark数据吗？没数据我不认可"},
    {"user": "Ji",      "text": "官方benchmark显示随机key查找快2-3倍"},
    {"user": "Nerissa", "text": "那行，用ahash"},
]

# 3. 重复决策（同一个问题反复讨论）
REPEAT_MSGS = [
    # 第一次讨论
    {"user": "Emmett",  "text": "时钟中断的栈需要单独分配吗？"},
    {"user": "Hobert",  "text": "需要，防止栈溢出导致 triple fault"},
    {"user": "Emmett",  "text": "好，知道了"},
    # 隔了一段时间重新讨论
    {"user": "Emmett",  "text": "我又看了下文档，时钟中断栈该分配多大？"},
    {"user": "Hobert",  "text": "之前说了，和Double Fault一样用4096"},
    {"user": "Emmett",  "text": "哦对，忘了，谢谢"},
    # 第三次
    {"user": "Emmett",  "text": "不好意思又来了，IST index从哪开始？"},
    {"user": "Hobert",  "text": "DOUBLE_FAULT_IST_INDEX=0，你时钟中断用1"},
    {"user": "Emmett",  "text": "好的这次记住了"},
    # 串口中断
    {"user": "Michell", "text": "串口的IRQ号是多少？"},
    {"user": "Floy",    "text": "IRQ4"},
    {"user": "Michell", "text": "谢了"},
    # ...
    {"user": "Michell", "text": "串口中断的IRQ是几来着，我又忘了"},
    {"user": "Floy",    "text": "IRQ4啊，文档里写了"},
    {"user": "Michell", "text": "啊对，看到了，记性不好"},
    # 进程相关
    {"user": "Noel",    "text": "ProcessManager的init要在什么时候调用？"},
    {"user": "Zack",    "text": "内存初始化之后，启用中断之前"},
    {"user": "Noel",    "text": "OK"},
    # ...
    {"user": "Noel",    "text": "我又忘了，ProcessManager init的时机是什么？"},
    {"user": "Zack",    "text": "内存之后、中断之前，文档说过了"},
    {"user": "Noel",    "text": "想起来了，抱歉"},
    # 页表
    {"user": "Stefan",  "text": "克隆页表是clone根节点还是整个树？"},
    {"user": "Pei",     "text": "根节点就够了，已确认过"},
    {"user": "Stefan",  "text": "好的"},
    # ...
    {"user": "Stefan",  "text": "等等，clone页表的时候帧分配器从哪获取？"},
    {"user": "Pei",     "text": "`get_frame_alloc_for_sure()` 函数"},
    {"user": "Stefan",  "text": "谢了"},
]

# 4. 噪声消息（闲聊、无关内容）
NOISE_MSGS = [
    {"user": "Kylie",   "text": "好饿，今天食堂有什么好吃的"},
    {"user": "Tanesha", "text": "今天食堂有红烧肉"},
    {"user": "Kylie",   "text": "冲冲冲"},
    {"user": "Zack",    "text": "+1"},
    {"user": "Danyel",  "text": "周末有人去打羽毛球吗"},
    {"user": "Kendall", "text": "我！好就没运动了"},
    {"user": "Danyel",  "text": "周六下午3点，老地方"},
    {"user": "Kendall", "text": "OK"},
    {"user": "Micki",   "text": "哈哈哈哈这个bug笑死我了"},
    {"user": "Micki",   "text": "手抖写了个死循环，结果IDE崩了"},
    {"user": "Rebecca", "text": "今天加班到几点？"},
    {"user": "Emmie",   "text": "我准备走了，代码编译过了"},
    {"user": "Rebecca", "text": "羡慕，我还卡在QEMU起不来"},
    {"user": "Emmie",   "text": "你看看是不是bootloader配置有问题"},
    {"user": "Rebecca", "text": "好我看看"},
    {"user": "Hobert",  "text": "有人试过在真机上跑YSOS吗"},
    {"user": "Emmett",  "text": "我不敢，怕黑屏"},
    {"user": "Hobert",  "text": "哈哈哈哈哈"},
    {"user": "Ji",      "text": "VSCode的Rust插件又崩了"},
    {"user": "Nerissa", "text": "建议你换Clion+rust插件，稳定很多"},
    {"user": "Ji",      "text": "CLion要钱啊"},
    {"user": "Nerissa", "text": "学生免费"},
    {"user": "Ji",      "text": "哦对，去申请一个"},
    {"user": "Tanesha", "text": "喝奶茶吗，我点外卖"},
    {"user": "Floy",    "text": "我要一杯四季春"},
    {"user": "Zack",    "text": "珍珠奶茶"},
    {"user": "Tanesha", "text": "好的，我下单了"},
    {"user": "Monroe",  "text": "今天好热啊，都快6月了"},
    {"user": "Tobias",  "text": "确实，空调开起来"},
    {"user": "Monroe",  "text": "实验室空调坏了..."},
    {"user": "Tobias",  "text": "……那没办法了"},
    {"user": "Maryalice", "text": "有人要拼单买书吗，那本RISC-V的书"},
    {"user": "Pei",     "text": "做OS的买RISC-V的书干啥"},
    {"user": "Maryalice", "text": "扩展知识面啊"},
    {"user": "Pei",     "text": "加我一个"},
    {"user": "Noel",    "text": "下周一交报告是吧"},
    {"user": "Kendall", "text": "对，0x02的实验报告"},
    {"user": "Noel",    "text": "赶紧写了"},
]

def main():
    messages = []

    # 生成时间戳（从 2026-05-07 09:00 开始）
    base_ts = 1778086800  # 2026-05-07 09:00:00 CST
    ts = base_ts

    # 按顺序组织话题，构建连续的对话
    topics = [
        # (topic_name, msgs, repeat_msgs, count)
        ("apic_init", DECISION_MSGS[:6] + CONFLICT_MSGS[:4], REPEAT_MSGS[:1], 0),
        ("clock_intr", DECISION_MSGS[6:14] + CONFLICT_MSGS[4:10], REPEAT_MSGS[3:12], 0),
        ("serial_intr", DECISION_MSGS[14:16] + REPEAT_MSGS[12:16], [], 0),
        ("lunch_noise", NOISE_MSGS[:4], [], 0),
        ("proc_stack", DECISION_MSGS[16:22] + CONFLICT_MSGS[10:18], REPEAT_MSGS[16:22], 0),
        ("scheduler", DECISION_MSGS[22:26] + CONFLICT_MSGS[18:24], [], 0),
        ("page_fault", DECISION_MSGS[26:28], [], 0),
        ("afternoon_noise", NOISE_MSGS[4:18], [], 0),
        ("gdt_tss", DECISION_MSGS[28:34], [], 0),
        ("thread_create", DECISION_MSGS[34:38], [], 0),
        ("process_exit", DECISION_MSGS[38:40], REPEAT_MSGS[22:30], 0),
        ("evening_noise", NOISE_MSGS[18:30], [], 0),
    ]

    for topic_name, topic_msgs, topic_repeat, _ in topics:
        for msg in topic_msgs:
            ts += random.randint(10, 60)
            messages.append({
                "user": msg["user"],
                "text": msg["text"],
                "ts": ts,
                "conversation_id": topic_name
            })

        for msg in topic_repeat:
            ts += random.randint(120, 180)  # 间隔2-3分钟
            messages.append({
                "user": msg["user"],
                "text": msg["text"],
                "ts": ts,
                "conversation_id": topic_name + "_repeat"
            })

    # 添加剩余噪声到各处
    remaining_noise = NOISE_MSGS[30:]
    for msg in remaining_noise:
        ts += random.randint(30, 120)
        messages.append({
            "user": msg["user"],
            "text": msg["text"],
            "ts": ts,
            "conversation_id": "noise"
        })

    # 限制到100条
    messages = messages[:100]

    # 按时间排序
    messages.sort(key=lambda m: m["ts"])

    # 只输出 user 和 text（和 test-messages.json 格式一致）
    simple_msgs = [{"user": m["user"], "text": m["text"]} for m in messages]

    output_path = "outputs/yatsenos-chat-100.json"
    with open(output_path, "w") as f:
        json.dump(simple_msgs, f, indent=2, ensure_ascii=False)

    print(f"Generated {len(simple_msgs)} messages → {output_path}")

    # 统计
    users = set(m["user"] for m in simple_msgs)
    print(f"Users: {len(users)} ({', '.join(sorted(users))})")
    print(f"Avg messages per user: {len(simple_msgs)/len(users):.1f}")

if __name__ == "__main__":
    main()
