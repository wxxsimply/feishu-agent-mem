from datetime import datetime, timezone
import time

# outputs/detect_state.json 里的 last_check
last_check_str = "2026-05-06T14:40:31.257658+08:00"
# Parse with timezone
last_check = datetime.fromisoformat(last_check_str.replace('Z', '+00:00'))
last_check_unix = int(last_check.timestamp())
print(f"lastCheck: {last_check} (Unix: {last_check_unix})")

# 文档的 update_time 从 +search 输出
doc_update_unix = 1778049493
doc_update_time = datetime.fromtimestamp(doc_update_unix, tz=timezone.utc).astimezone()
print(f"Doc update_time: {doc_update_time} (Unix: {doc_update_unix})")

# 比较一下
print(f"docUpdateTime < lastCheck? {doc_update_time < last_check}")
print(f"docUpdateUnix < lastCheck.Unix()? {doc_update_unix < last_check_unix}")

# 现在时间
now = datetime.now()
now_unix = int(time.time())
print(f"Now: {now} (Unix: {now_unix})")
