import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

mount_time = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        labels = r["labels"]
        if r["taskName"] == "patch" or r["taskName"] == "di3fs":
            name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
            valueName = "{}-comp-{}-enc-{}".format(r["taskName"], labels["compressionMode"], labels["deltaEncoding"])
            if name not in mount_time:
                mount_time[name] = {}
            mount_time[name][valueName] = r["elapsedMilliseconds"]


labels = []
for task in ["patch", "di3fs"]:
    for comp in ["bzip2"]:
        for enc in ["bsdiffx", "xdelta3"]:
            labels.append("{}-comp-{}-enc-{}".format(task, comp, enc))

plt.rcParams["figure.figsize"] = (20,10)
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax = [ax]
ax[0].set_ylabel("Milliseconds")
ax[0].set_title("binary_diff_time")

keys = list(mount_time.keys())
data_num = len(labels)
factor = (data_num+1) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(mount_time[k][l])
    ax[0].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l)

plt.xlim(0, (len(keys)-1)*factor+BAR_WIDTH*data_num)
plt.xticks([x*factor+BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x.replace("-", "-\n", 1) for x in keys])
plt.tight_layout()
plt.legend()

plt.savefig(sys.argv[2])