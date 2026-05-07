#!/usr/bin/env python3
"""MCP client: create two conflicting decisions, observe git changes."""
import json
import subprocess
import sys
import time

MCP_SERVER = "./bin/mcp-server"
WORK_DIR = "/Users/halllo/openclaw-workspace/feishu-agent-mem"


def run(cmd, cwd=None):
    subprocess.run(cmd, shell=True, check=False, cwd=cwd or WORK_DIR)


class MCPClient:
    def __init__(self):
        self.proc = subprocess.Popen(
            [MCP_SERVER],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            cwd=WORK_DIR,
            text=True,
            bufsize=1,
        )
        self.msg_id = 0

    def send(self, method, params=None):
        self.msg_id += 1
        req = {"jsonrpc": "2.0", "id": self.msg_id, "method": method}
        if params:
            req["params"] = params
        req_str = json.dumps(req, ensure_ascii=False) + "\n"
        print(f"  >>> {method}", file=sys.stderr)
        self.proc.stdin.write(req_str)
        self.proc.stdin.flush()
        line = self.proc.stdout.readline()
        if not line:
            print("  <<< NO RESPONSE", file=sys.stderr)
            return None
        resp = json.loads(line)
        if resp.get("error"):
            print(f"  <<< ERROR: {resp['error']}", file=sys.stderr)
        else:
            result = resp.get("result", {})
            text = ""
            if "content" in result:
                for c in result["content"]:
                    if c.get("type") == "text":
                        text = c.get("text", "")
                        break
            if text:
                print(f"  <<< {text[:200]}", file=sys.stderr)
            return result
        return resp

    def close(self):
        self.proc.terminate()
        self.proc.wait()


def show_git_state(label):
    print(f"\n{'='*60}", file=sys.stderr)
    print(f"  {label}", file=sys.stderr)
    print(f"{'='*60}", file=sys.stderr)
    run("cd data && echo '--- Branches ---' && git branch -a && "
        "echo '' && echo '--- DAG ---' && git log --graph --oneline --all --decorate")
    # Show decision files per branch
    branches = subprocess.run(
        "cd data && git branch | grep 'decision/' | tr -d ' *'",
        shell=True, capture_output=True, text=True, cwd=WORK_DIR
    ).stdout.strip().split("\n")
    for b in branches:
        if not b:
            continue
        files = subprocess.run(
            f"cd data && git ls-tree -r --name-only {b} | head -3",
            shell=True, capture_output=True, text=True, cwd=WORK_DIR
        ).stdout.strip()
        for f in files.split("\n"):
            if f and f.endswith(".md") and "L0_RULES" not in f and "DEC-000" not in f:
                content = subprocess.run(
                    f"cd data && git show {b}:{f}",
                    shell=True, capture_output=True, text=True, cwd=WORK_DIR
                ).stdout.strip()
                # Show only yaml frontmatter
                if "---" in content:
                    parts = content.split("---")
                    if len(parts) >= 3:
                        print(f"\n  [{b}] {f}:", file=sys.stderr)
                        for line in parts[1].strip().split("\n")[:8]:
                            print(f"    {line}", file=sys.stderr)


def main():
    print("\n=== MCP: 创建两个冲突决策 + Git 观察 ===\n", file=sys.stderr)

    # Clean start
    run("rm -rf data && rm -rf bin/mcp-server")
    run("go build -o bin/mcp-server ./cmd/mcp-server/main.go")
    print("\n[0] Clean start, compiled mcp-server\n", file=sys.stderr)

    client = MCPClient()

    # Initialize
    print("\n[1] Initializing MCP connection...", file=sys.stderr)
    client.send("initialize", {
        "protocolVersion": "2024-11-05", "capabilities": {},
        "clientInfo": {"name": "test", "version": "1.0"},
    })
    client.send("notifications/initialized")

    show_git_state("初始状态: 只有 Dummy")

    # Create Decision A
    print("\n[2] Create DECISION A: 数据库采用 PostgreSQL 15...", file=sys.stderr)
    client.send("tools/call", {
        "name": "create_decision",
        "arguments": {
            "title": "数据库采用 PostgreSQL 15",
            "decision": "经过架构评审，决定采用 PostgreSQL 15 作为主数据库，支持复杂查询和 MVCC",
            "rationale": "PostgreSQL 15 在复杂查询性能、MVCC、扩展性方面优于 MySQL，适合业务快速增长",
            "topic": "数据库架构",
            "impact_level": "major",
        },
    })
    time.sleep(2)
    show_git_state("决策 A 创建后")

    # Create Decision B (conflicting with A)
    print("\n[3] Create DECISION B: 数据库采用 MySQL 8.0（与 A 冲突）...", file=sys.stderr)
    client.send("tools/call", {
        "name": "create_decision",
        "arguments": {
            "title": "数据库改为 MySQL 8.0",
            "decision": "基于运维团队能力评估，决定继续使用 MySQL 8.0，采用读写分离架构，待后续再评估分布式方案",
            "rationale": "现有团队对 MySQL 运维经验丰富，PostgreSQL 学习成本高。MySQL 8.0 读写分离可满足当前需求",
            "topic": "数据库架构",
            "impact_level": "major",
        },
    })
    time.sleep(2)
    show_git_state("决策 B 创建后")

    # Show full state
    print(f"\n{'='*60}", file=sys.stderr)
    print("  最终状态", file=sys.stderr)
    print(f"{'='*60}", file=sys.stderr)
    run("cd data && echo '--- ALL BRANCHES ---' && git branch -a && "
        "echo '' && echo '--- FULL DAG ---' && git log --graph --oneline --all --decorate")
    # Show decision file contents
    run("cd data && find decisions -name '*.md' ! -name 'DEC-000*' -exec echo '=== {} ===' \\; -exec head -15 {} \\;")

    client.close()
    print("\n=== Done ===", file=sys.stderr)


if __name__ == "__main__":
    main()
