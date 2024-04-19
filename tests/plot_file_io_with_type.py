import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

fe_types = {
    0: "new",
    1: "same",
    2: "diff",
}
files = {}
with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        path = r["path"]
        entryType = r["fileEntryBType"]
        labels = r["labels"]
        imageName = labels["imageName"]
        new = labels["new"]
        old = labels["old"]
        tag = "{}:{}-{}".format(imageName, old, new)
        if tag not in files:
            files[tag] = {}
        files[tag][path] = entryType

stat_open = {}
stat_read = {}
stat_openAndRead = {}

with open(sys.argv[2]) as f:
    for l in f.readlines():
        r = json.loads(l)
        taskName = r["taskName"]
        fileSize = r["size"]
        labels = r["labels"]
        imageName = labels["imageName"]
        new = labels["new"]
        old = labels["old"]
        path = labels["path"]
        root = labels["root"]
        path = path.replace(root, '', 1)
        pathLabel = labels["pathLabel"]
        # ignore 0-bytes file
        if fileSize == 0:
            continue
        if pathLabel != "di3fs":
            continue
        if labels["count"] != "0":
            continue
        tag = "{}:{}-{}".format(imageName, old, new)
        entryType = files[tag][path]

        tag = "EntryType {}".format(fe_types[entryType])
        elapsedUS = r["elapsedMicroseconds"]
        if tag not in stat_open:
            # [path, fileSize, efficiency]
            stat_open[tag] = [[], []]
            stat_read[tag] = [[], []]
            stat_openAndRead[tag] = [[], []]
        if taskName == "open":
            stat_open[tag][0].append(fileSize)
            stat_open[tag][1].append(elapsedUS)
        if taskName == "read":
            stat_read[tag][0].append(fileSize)
            stat_read[tag][1].append(elapsedUS)
        if taskName == "open+read":
            stat_openAndRead[tag][0].append(fileSize)
            stat_openAndRead[tag][1].append(elapsedUS)


labels = []
plt.rcParams["figure.figsize"] = (20,10)
fig, ax = plt.subplots(nrows=3, ncols=1, sharex=True)
ax[0].set_ylabel("Elapsed (microseconds)")
ax[0].set_title("File I/O (open)")
ax[1].set_ylabel("Elapsed (microseconds)")
ax[1].set_title("File I/O (read)")
ax[2].set_ylabel("Elapsed (microseconds)")
ax[2].set_title("File I/O (open+read)")
ax[2].set_xlabel("File size (bytes)")

for tag in stat_openAndRead:
    value = stat_open[tag]
    ax[0].scatter(value[0], value[1], label=tag)
    value = stat_read[tag]
    ax[1].scatter(value[0], value[1], label=tag)
    value = stat_openAndRead[tag]
    ax[2].scatter(value[0], value[1], label=tag)

ax[0].legend()
ax[1].legend()
ax[2].legend()
plt.tight_layout()

plt.savefig(sys.argv[3])