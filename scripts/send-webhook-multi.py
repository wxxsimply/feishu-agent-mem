#!/usr/bin/env python3
import json
import time
import sys
import os
import requests

def load_webhooks(webhooks_file):
    """Load webhooks from file: "Name url" per line"""
    webhooks = {}
    with open(webhooks_file) as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            parts = line.split()
            if len(parts) >= 2:
                name = parts[0]
                url = parts[1]
                webhooks[name] = url
    return webhooks

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
        print(f"Usage: {sys.argv[0]} <json_file> <webhooks_file> [start_index] [count]")
        print(f"Example: {sys.argv[0]} ../outputs/test-messages.json ../webhooks.md 0 100")
        sys.exit(1)

    json_path = sys.argv[1]
    webhooks_file = sys.argv[2]
    start_index = int(sys.argv[3]) if len(sys.argv) > 3 else 0
    count = int(sys.argv[4]) if len(sys.argv) > 4 else None

    webhooks = load_webhooks(webhooks_file)
    print(f"Loaded {len(webhooks)} webhooks")
    print(f"Users: {list(webhooks.keys())[:5]}...")
    print()

    with open(json_path, 'r') as f:
        messages = json.load(f)

    end_index = start_index + count if count is not None else len(messages)
    messages_to_send = messages[start_index:end_index]

    print(f"Sending {len(messages_to_send)} messages")
    print(f"Start index: {start_index}")
    print("-" * 60)

    success_count = 0
    for i, msg in enumerate(messages_to_send):
        user = msg['user']
        text = msg['text']

        webhook_url = webhooks.get(user)
        if not webhook_url:
            print(f"[{i + start_index + 1}/{len(messages)}] Skipping {user} - no webhook found")
            continue

        # Send only the message content, without [user] prefix
        # (since it's coming from that user's bot already)
        print(f"[{i + start_index + 1}/{len(messages)}] {user}: ", end="")

        if send_webhook(webhook_url, text):
            print(f"{text[:45]}...")
            success_count += 1
        else:
            print("Failed!")

        # 1 second delay between messages
        time.sleep(1)

    print("-" * 60)
    print(f"Done! Sent {success_count}/{len(messages_to_send)} messages successfully")

if __name__ == '__main__':
    main()
