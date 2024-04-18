import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

merge_time = {}
merge_size = {}
binary_diff_time ={}
binary_diff_size ={}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "merge":
            continue
        labels = r["labels"]
        name = "{}-{}".format(labels["imageName"], labels["out"])
        valueName = "th-{}-sched-{}-comp-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"])
        if name not in merge_time:
            merge_time[name] = {}
            merge_size[name] = {}
        merge_time[name][valueName] = r["elapsedMilliseconds"]
        merge_size[name][valueName] = r["size"]

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "diff":
            continue
        labels = r["labels"]
        if labels["mode"] != "binary-diff":
            continue

        name = "{}-{}".format(labels["imageName"], labels["out"])
        if name not in merge_time:
            continue
        valueName = "th-{}-sched-{}-comp-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"])
        if name not in binary_diff_time:
            binary_diff_time[name] = {}
            binary_diff_size[name] = {}
        binary_diff_time[name][valueName] = r["elapsedMilliseconds"]
        binary_diff_size[name][valueName] = r["size"]

labels = []
for th in ["1", "8"]:
    for sched in ["size-ordered"]:
        if th == "1" and sched != "none":
            continue
        for comp in ["bzip2"]:
            labels.append("th-{}-sched-{}-comp-{}".format(th, sched, comp))

plt.rcParams["figure.figsize"] = (10,10)
fig, ax = plt.subplots(nrows=4, ncols=1, sharex=True)
ax[0].set_ylabel("Milliseconds")
ax[0].set_title("merge_time")
ax[1].set_ylabel("Milliseconds")
ax[1].set_title("binary_diff_time")
ax[2].set_ylabel("bytes")
ax[2].set_title("merge_size")
ax[3].set_ylabel("bytes")
ax[3].set_title("binary_diff_size")

keys = list(merge_time.keys())
data_num = len(labels)
factor = (data_num+1) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(merge_time[k][l])
    ax[0].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(binary_diff_time[k][l])
    ax[1].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(merge_size[k][l])
    ax[2].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(binary_diff_size[k][l])
    ax[3].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

plt.xlim(0, (len(keys)-1)*factor+BAR_WIDTH*data_num)
plt.xticks([x*factor+BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x.replace("-", "-\n", 2) for x in keys])
plt.tight_layout()
plt.legend()

plt.savefig(sys.argv[2])