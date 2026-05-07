#!/usr/bin/env python3
import xml.etree.ElementTree as ET
import json
import sys

def parse_xml(xml_path, limit=100):
    tree = ET.parse(xml_path)
    root = tree.getroot()

    messages = []
    for msg in root.findall('message')[:limit]:
        ts = msg.find('ts').text
        user = msg.find('user').text
        text = msg.find('text').text
        conversation_id = msg.get('conversation_id')

        messages.append({
            'ts': ts,
            'user': user,
            'text': text,
            'conversation_id': conversation_id
        })

    return messages

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <xml_file> [limit] [output_json]")
        print(f"Example: {sys.argv[0]} ../chat-data/Software-related-Slack-Chats-with-Disentangled-Conversations/data/pythondev/2018/merged-pythondev-help.xml 100 test-messages.json")
        sys.exit(1)

    xml_path = sys.argv[1]
    limit = int(sys.argv[2]) if len(sys.argv) > 2 else 100
    output_path = sys.argv[3] if len(sys.argv) > 3 else 'test-messages.json'

    messages = parse_xml(xml_path, limit)

    with open(output_path, 'w') as f:
        json.dump(messages, f, indent=2, ensure_ascii=False)

    print(f"Extracted {len(messages)} messages to {output_path}")
