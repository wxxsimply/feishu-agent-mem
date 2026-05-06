#!/usr/bin/env python3
import json
import subprocess
import time
import sys
import os

def send_message(chat_id, text):
    cmd = [
        'lark-cli',
        'im',
        '+messages-send',
        '--chat-id', chat_id,
        '--text', text,
        '--as', 'bot'
    ]

    result = subprocess.run(cmd, capture_output=True, text=True)

    if result.returncode != 0:
        print(f"Error sending message: {result.stderr}", file=sys.stderr)
        return False

    print(f"Sent: {text[:50]}...")
    return True

def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <json_file> <chat_id> [start_index] [count]")
        print(f"Example: {sys.argv[0]} test-messages.json oc_test_chat 0 10")
        print(f"Example (single message): {sys.argv[0]} test-messages.json oc_test_chat 0 1")
        sys.exit(1)

    json_path = sys.argv[1]
    chat_id = sys.argv[2]
    start_index = int(sys.argv[3]) if len(sys.argv) > 3 else 0
    count = int(sys.argv[4]) if len(sys.argv) > 4 else None

    with open(json_path, 'r') as f:
        messages = json.load(f)

    end_index = start_index + count if count is not None else len(messages)
    messages_to_send = messages[start_index:end_index]

    print(f"Sending {len(messages_to_send)} messages to {chat_id}")
    print(f"Start index: {start_index}")
    print("-" * 50)

    success_count = 0
    for i, msg in enumerate(messages_to_send):
        user = msg['user']
        text = msg['text']
        full_text = f"[{user}] {text}"

        print(f"[{i + start_index + 1}/{len(messages)}] ", end="")

        if send_message(chat_id, full_text):
            success_count += 1

        # Small delay between messages
        time.sleep(0.5)

    print("-" * 50)
    print(f"Done! Sent {success_count}/{len(messages_to_send)} messages successfully")

if __name__ == '__main__':
    main()
