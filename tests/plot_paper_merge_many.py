import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

mergedDimgs = [4, 8, 12, 16, 20]
merge_time ={}
merge_time_agg = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] == "merge":
            labels = r["labels"]
            mode = labels["mergeMode"]
            if mode not in merge_time:
                merge_time[mode] = {}
            dimgNum = int(labels["mergedCdimgsNum"])
            if dimgNum not in merge_time[mode]:
                merge_time[mode][dimgNum] = []
            merge_time[mode][dimgNum].append(r["elapsedMilliseconds"] / 1000)


for mode in ["linear", "bisect"]:
    merge_time_agg[mode] = []
    for num in mergedDimgs:
        rs = merge_time[mode][num]
        merge_time_agg[mode].append(sum(rs) / len(rs))

plt.rcParams["figure.figsize"] = (5,6)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Time to merge (Seconds)")
ax.set_xlabel("Number of merged delta bundles")
for l in merge_time_agg:
    ax.plot(mergedDimgs, merge_time_agg[l], marker="o", linestyle="dashed", label=l)

cycle = plt.rcParams['axes.prop_cycle'].by_key()['color']
plt.legend()
plt.tight_layout()

plt.savefig("eval-merge-many.pdf")