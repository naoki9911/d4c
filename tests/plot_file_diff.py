import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

diff_time = {}
diff_size = {}
merge_time = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        labels = r["labels"]
        if r["taskName"] == "diff-per-file":
            name = "type-{}-comp-{}".format(labels["type"], labels["compressionMode"])
            if name not in diff_time:
                diff_time[name] = [[],[]]
                diff_size[name] = [[], []]
            diff_time[name][0].append(r["size"])
            diff_time[name][1].append(r["elapsedMilliseconds"])
            diff_size[name][0].append(r["size"])
            diff_size[name][1].append(int(labels["compressedSize"]))

            if labels["type"] == "file_diff" and "obj" in labels and labels["obj"] == "merge":
                name = "mode-diff-comp-{}".format(labels["compressionMode"])
                if name not in merge_time:
                    merge_time[name] = [[], []]
                merge_time[name][0].append(r["size"])
                merge_time[name][1].append(r["elapsedMilliseconds"])

        elif r["taskName"] == "merge-per-file":
            mode = labels["mergeMode"]
            if mode == "copy-upper" or mode == "copy-lower":
                continue
            name = "mode-{}-comp-{}".format(mode, labels["compressionMode"])
            if name not in merge_time:
                merge_time[name] = [[], []]
            merge_time[name][0].append(r["size"])
            merge_time[name][1].append(r["elapsedMilliseconds"])

labels = []
plt.rcParams["figure.figsize"] = (20,10)
fig, ax = plt.subplots(nrows=3, ncols=1, sharex=True)
ax[0].set_ylabel("Milliseconds")
ax[0].set_title("diff_time")
ax[1].set_ylabel("bytes")
ax[1].set_title("diff_size")
ax[2].set_ylabel("Milliseconds")
ax[2].set_title("merge_time")

for name in diff_time:
    d_time = diff_time[name]
    d_size = diff_size[name]
    ax[0].scatter(d_time[0], d_time[1], label=name)
    ax[1].scatter(d_size[0], d_size[1], label=name)
for name in merge_time:
    m_time = merge_time[name]
    ax[2].scatter(m_time[0], m_time[1], label=name)

ax[0].legend()
ax[1].legend()
ax[2].legend()
plt.tight_layout()

plt.savefig(sys.argv[2])