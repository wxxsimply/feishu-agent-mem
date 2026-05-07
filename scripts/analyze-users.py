#!/usr/bin/env python3
import json
from collections import Counter

with open('outputs/test-messages.json') as f:
    messages = json.load(f)

users = [msg['user'] for msg in messages]
user_counts = Counter(users)

print(f"总消息数: {len(messages)}")
print(f"不同用户数: {len(user_counts)}")
print()
print("用户发言统计:")
print("-" * 40)
for user, count in user_counts.most_common():
    print(f"  {user:20s}: {count:3d} 条")
print("-" * 40)
print(f"Top 5 users: {[u for u, _ in user_counts.most_common(5)]}")
