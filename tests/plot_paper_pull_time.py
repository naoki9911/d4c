import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

pull_time ={}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "pull":
            continue
        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        if labels["mode"] == "binary-diff":
            valueName = "th-{}-sched-{}-comp-{}-enc-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"], labels["deltaEncoding"])
            if name not in pull_time:
                pull_time[name] = {}
            if valueName not in pull_time[name]:
                pull_time[name][valueName] = []
            pull_time[name][valueName].append(r["elapsedMilliseconds"] / 1000)
        elif labels["mode"] == "file-diff":
            valueName = "th-{}-sched-{}-comp-{}-enc-file".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"])
            if name not in pull_time:
                pull_time[name] = {}
            if valueName not in pull_time[name]:
                pull_time[name][valueName] = []
            pull_time[name][valueName].append(r["elapsedMilliseconds"] / 1000)
        else:
            raise Exception("invalid mode {}".format(labels["mode"]))

labels = []
for th in ["8"]:
    for sched in ["none"]:
        for comp in ["bzip2"]:
            for enc in [("file", "file-by-file"), ("xdelta3", "xdelta3"), ("bsdiffx", "bsdiff")]:
                labels.append((("th-{}-sched-{}-comp-{}-enc-{}".format(th, sched, comp, enc[0])), enc[1]))

plt.rcParams["figure.figsize"] = (12,4)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Time to pull (Seconds)")
ax2 = ax.twinx()
ax2.set_ylabel("Time to pull (Seconds, pytorch)", fontsize=14)

keys = list(pull_time.keys())
keys = []
keys.append(("postgres-13.1-13.2", "\n.1→.2"))
keys.append(("postgres-13.2-13.3", "postgres\n.2→.3"))
keys.append(("postgres-13.1-13.3", "\n.1→.3"))
keys.append(("nginx-1.23.1-1.23.2", "\n.1→.2"))
keys.append(("nginx-1.23.2-1.23.3", "nginx\n.2→.3"))
keys.append(("nginx-1.23.1-1.23.3", "\n.1→.3"))
keys.append(("redis-7.0.5-7.0.6", "\n.5→.6"))
keys.append(("redis-7.0.6-7.0.7", "redis\n.6→.7"))
keys.append(("redis-7.0.5-7.0.7", "\n.5→.7"))

keys2 = []
keys2.append(("pytorch-2.2.0-cuda12.1-cudnn8-runtime-2.2.1-cuda12.1-cudnn8-runtime", "\n.0→.1"))
keys2.append(("pytorch-2.2.1-cuda12.1-cudnn8-runtime-2.2.2-cuda12.1-cudnn8-runtime", "pytorch\n.1→.2"))
keys2.append(("pytorch-2.2.0-cuda12.1-cudnn8-runtime-2.2.2-cuda12.1-cudnn8-runtime", "\n.0→.2"))

data_num = len(labels)
factor = (data_num+0.5) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        v = pull_time[k[0]][l[0]]
        value.append(sum(v) / len(v))
    p = ax.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])
    ax.bar_label(p, fmt="%.1f", fontsize=14, rotation=90, padding=2)
ax.set_ylim(0, 8)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys2:
        v = pull_time[k[0]][l[0]]
        value.append(sum(v) / len(v))
    p = ax2.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(len(keys), len(keys) + len(keys2))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])
    ax2.bar_label(p, fmt="%.1f", fontsize=14, rotation=90, padding=2)
ax2.set_ylim(0, 190)

keys.extend(keys2)
ax.legend(loc='upper center')
ax.tick_params()
plt.xlim(0, (len(keys)-1)*factor+ (int((len(keys)-1)/3) * BAR_WIDTH/3)+BAR_WIDTH*data_num)
plt.xticks([x*factor+ (int(x/3)*BAR_WIDTH/3) + BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x[1] for x in keys])
plt.tight_layout()

plt.savefig("eval-pull-time.pdf")