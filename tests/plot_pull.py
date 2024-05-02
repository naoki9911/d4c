import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

binary_pull_time ={}
file_pull_time = {}
binary_download_time ={}
file_download_time = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "pull":
            continue
        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        valueName = "th-{}-sched-{}-comp-{}-enc-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"], labels["deltaEncoding"])
        if labels["mode"] == "binary-diff":
            if name not in binary_pull_time:
                binary_pull_time[name] = {}
            binary_pull_time[name][valueName] = r["elapsedMilliseconds"]
        elif labels["mode"] == "file-diff":
            if name not in file_pull_time:
                file_pull_time[name] = {}
            file_pull_time[name][valueName] = r["elapsedMilliseconds"]
        else:
            raise Exception("invalid mode {}".format(labels["mode"]))

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "pull-download":
            continue
        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        valueName = "th-{}-sched-{}-comp-{}-enc-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"], labels["deltaEncoding"])
        if labels["mode"] == "binary-diff":
            if name not in binary_download_time:
                binary_download_time[name] = {}
            binary_download_time[name][valueName] = r["elapsedMilliseconds"]
        elif labels["mode"] == "file-diff":
            if name not in file_download_time:
                file_download_time[name] = {}
            file_download_time[name][valueName] = r["elapsedMilliseconds"]
        else:
            raise Exception("invalid mode {}".format(labels["mode"]))

labels = []
for th in ["1", "8"]:
    for sched in ["none"]:
        for comp in ["bzip2"]:
            for enc in ["bsdiffx", "xdelta3"]:
                labels.append("th-{}-sched-{}-comp-{}-enc-{}".format(th, sched, comp, enc))

plt.rcParams["figure.figsize"] = (20,10)
fig, ax = plt.subplots(nrows=4, ncols=1, sharex=True)
ax[0].set_ylabel("Milliseconds")
ax[0].set_title("binary_pull_time")
ax[1].set_ylabel("Milliseconds")
ax[1].set_title("file_pull_time")
ax[2].set_ylabel("Milliseconds")
ax[2].set_title("binary_download_time")
ax[3].set_ylabel("Milliseconds")
ax[3].set_title("file_download_time")

keys = list(binary_pull_time.keys())
data_num = len(labels)
factor = (data_num+1) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(binary_pull_time[k][l])
    ax[0].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(file_pull_time[k][l])
    ax[1].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(binary_download_time[k][l])
    ax[2].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(file_download_time[k][l])
    ax[3].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

plt.xlim(0, (len(keys)-1)*factor+BAR_WIDTH*data_num)
plt.xticks([x*factor+BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x.replace("-", "-\n", 1) for x in keys])
plt.tight_layout()
plt.legend()

plt.savefig(sys.argv[2])