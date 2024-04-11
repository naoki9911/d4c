import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

compare = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        labels = r["labels"]
        imageName = labels["imageName"]
        new = labels["new"]
        old = labels["old"]
        path = r["path"]
        fileSize = r["fileSize"]
        # ignore 0-bytes file
        if fileSize == 0:
            continue
        fileDiffSize = r["fileEntryACompressionSize"]
        binaryDiffSize = r["fileEntryBCompressionSize"]
        efficiency = float(binaryDiffSize) / float(fileDiffSize)
        tag = "{}:{}-{}".format(imageName, old, new)
        if tag not in compare:
            # [path, fileSize, efficiency]
            compare[tag] = [[], [], []]
        compare[tag][0].append(path)
        compare[tag][1].append(fileSize)
        compare[tag][2].append(efficiency)

labels = []
plt.rcParams["figure.figsize"] = (20,10)
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax = [ax]
ax[0].set_ylabel("Efficiency")
ax[0].set_title("Compare")

for tag in compare:
    value = compare[tag]
    ax[0].scatter(value[1], value[2], label=tag)

ax[0].legend()
plt.tight_layout()

plt.savefig(sys.argv[2])