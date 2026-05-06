#!/usr/bin/env python3
import json
import time
import sys
import os
import requests

def send_webhook(webhook_url, text):
    """Send message via lark webhook"""
    data = {
        "msg_type": "text",
        "content": {
            "text": text
        }
    }

    try:
        response = requests.post(webhook_url, json=data, timeout=10)
        response.raise_for_status()
        result = response.json()

        if result.get('code') == 0:
            return True
        else:
            print(f"Error: {result}", file=sys.stderr)
            return False
    except Exception as e:
        print(f"Request failed: {e}", file=sys.stderr)
        return False

def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <json_file> <webhook_url> [start_index] [count]")
        print(f"Example: {sys.argv[0]} ../outputs/test-messages.json https://open.feishu.cn/open-apis/bot/v2/hook/xxx 0 10")
        sys.exit(1)

    json_path = sys.argv[1]
    webhook_url = sys.argv[2]
    start_index = int(sys.argv[3]) if len(sys.argv) > 3 else 0
    count = int(sys.argv[4]) if len(sys.argv) > 4 else None

    with open(json_path, 'r') as f:
        messages = json.load(f)

    end_index = start_index + count if count is not None else len(messages)
    messages_to_send = messages[start_index:end_index]

    print(f"Sending {len(messages_to_send)} messages via webhook")
    print(f"Start index: {start_index}")
    print("-" * 50)

    success_count = 0
    for i, msg in enumerate(messages_to_send):
        user = msg['user']
        text = msg['text']
        full_text = f"[{user}] {text}"

        print(f"[{i + start_index + 1}/{len(messages)}] ", end="")

        if send_webhook(webhook_url, full_text):
            print(f"Sent: {text[:40]}...")
            success_count += 1
        else:
            print("Failed!")

        # Small delay between messages to avoid rate limit
        time.sleep(0.5)

    print("-" * 50)
    print(f"Done! Sent {success_count}/{len(messages_to_send)} messages successfully")

if __name__ == '__main__':
    main()
