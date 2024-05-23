#!/usr/bin/env python3

"""
Parses the log output of grpc_proxy::inspect_io::* logs
"""

from datetime import datetime as dt

with open("long/rust/start-0.stderr") as f: lines = f.readlines()

actions, ids, avgs = {}, {}, []
for (i, line) in enumerate(lines):
    i += 1
    line = line.strip()
    if line.find("grpc_proxy::inspect_io::") == -1:
        if line.startswith("Avg:"):
            avgs.append(float(line.split(" ")[1]) / 1000)
        continue
    parts = line.split(" - ")
    prefix, msg = parts[0], parts[1]

    time_str = prefix.split(" ")[0]
    time_str = time_str.rsplit("-", 1)[0]
    parts = time_str.rsplit(":", 1)
    secs = float(parts[1])
    time = dt.strptime(parts[0], "%Y-%m-%dT%H:%M").timestamp() + secs
    
    parts = msg.split(" ")
    id, start, action = int(parts[0]), parts[1] == "start", parts[2]
    prev = ids.pop(id, None)
    if prev is None:
        ids.setdefault(id, time)
    else:
        actions.setdefault(action, [])
        actions[action].append(time - prev)

print("Values in seconds\n")

acts = list(sorted(actions.keys()))
ops_total_sum, ops_total_n = 0, 0
proxy_sum, proxy_n, proxy_avg = 0, 0, 0
for action in acts:
    times = actions[action]
    s, n = sum(times), len(times)
    avg = s / n
    if action == "proxy":
        proxy_sum, proxy_n, proxy_avg = s, n, avg
        continue
    print(f"{action}: SUM={s}, AVG={avg}, N = {n}")
    ops_total_sum += s
    ops_total_n += n

print()
print(f"proxy: SUM={proxy_sum}, AVG={proxy_avg}, N={proxy_n}")

ops_total_avg = ops_total_sum / ops_total_n
print()
print(f"Total Ops: SUM={ops_total_sum}, AVG={ops_total_avg}, N={ops_total_n}")

avgs.sort()
l = len(avgs)
mid = l // 2
avgs_med = avgs[mid] if l % 2 == 0 else (avgs[mid - 1] + avgs[mid]) / 2
avgs_mean = sum(avgs) / l
avgs_min, avgs_max = min(avgs), max(avgs)
print()
print(f"Average Proxy Request:")
print(f"\tMIN={avgs_min}, MAX={avgs_max}")
print(f"\tMED={avgs_med}, MEAN={avgs_mean}")
