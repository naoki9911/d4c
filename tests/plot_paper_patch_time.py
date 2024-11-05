import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

patch_time ={}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "patch" and r["taskName"] != "di3fs":
            continue
        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        if labels["mode"] != "binary-diff":
            continue
        valueName = "{}-th-{}-sched-{}-comp-{}-enc-{}".format(r["taskName"], labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"], labels["deltaEncoding"])
        if name not in patch_time:
            patch_time[name] = {}
        if valueName not in patch_time[name]:
            patch_time[name][valueName] = []
        patch_time[name][valueName].append(r["elapsedMilliseconds"] / 1000)

labels = []
for th in ["8"]:
    for sched in ["none"]:
        for comp in ["bzip2"]:
            for enc in [("xdelta3", "xdelta3"), ("bsdiffx", "bsdiff")]:
                for task in ["patch", "di3fs"]:
                    labels.append((("{}-th-{}-sched-{}-comp-{}-enc-{}".format(task, th, sched, comp, enc[0])), "{}({})".format(enc[1], task.replace("di3fs", "Di3FS"))))

plt.rcParams["figure.figsize"] = (12,4)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Time to mount bundles (Seconds)", fontsize=14)
ax2 = ax.twinx()
ax2.set_ylabel("Time to mount bundles (Seconds, pytorch)", fontsize=11)

keys = list(patch_time.keys())
keys = []
keys.append(("postgres-13.1-13.2", "\n.1→.2"))
keys.append(("postgres-13.2-13.3", "\n.2→.3"))
#keys.append(("postgres-13.1-13.3", "\n.1→.3"))
keys.append(("nginx-1.23.1-1.23.2", "\n.1→.2"))
keys.append(("nginx-1.23.2-1.23.3", "\n.2→.3"))
#keys.append(("nginx-1.23.1-1.23.3", "\n.1→.3"))
keys.append(("redis-7.0.5-7.0.6", "\n.5→.6"))
keys.append(("redis-7.0.6-7.0.7", "\n.6→.7"))
#keys.append(("redis-7.0.5-7.0.7", "\n.5→.7"))

keys2 = []
keys2.append(("pytorch-2.2.0-cuda12.1-cudnn8-runtime-2.2.1-cuda12.1-cudnn8-runtime", "\n.0→.1"))
keys2.append(("pytorch-2.2.1-cuda12.1-cudnn8-runtime-2.2.2-cuda12.1-cudnn8-runtime", "\n.1→.2"))

data_num = len(labels)
factor = (data_num+0.5) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        v = patch_time[k[0]][l[0]]
        value.append(sum(v) / len(v))
    p = ax.bar([(x*factor + (int(x/2)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])
    ax.bar_label(p, fmt="%.1f", fontsize=14, rotation=90, padding=2)
ax.set_ylim(0, 6.5)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys2:
        v = patch_time[k[0]][l[0]]
        value.append(sum(v) / len(v))
    p = ax2.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(len(keys), len(keys) + len(keys2))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])
    ax2.bar_label(p, fmt="%.1f", fontsize=14, rotation=90, padding=2)
ax2.set_ylim(0, 82)

ax.legend(loc='upper center', fontsize=14)
ax.tick_params()
keys.extend(keys2)
xlim = (len(keys)-1)*factor+ (int((len(keys)-1)/2) * BAR_WIDTH/3)+BAR_WIDTH*data_num
plt.xlim(0, xlim)
xticks_cord = [x*factor+ (int(x/2)*BAR_WIDTH/3) + BAR_WIDTH*data_num/2 for x in range(0, len(keys))]
xticks_cord.extend((x+1/2)*factor+ (int(x/2+1/4)*BAR_WIDTH/3) + BAR_WIDTH*data_num/2 for x in range(0, int(len(keys)), 2))
xticks_labels = [x[1] for x in keys]
xticks_labels.extend(["postgres", "nginx", "redis", "pytorch"])
plt.xticks(xticks_cord, xticks_labels)
plt.tight_layout()

plt.savefig("eval-patch-time.pdf")