import matplotlib.pyplot as plt
import numpy as np 
import json
import sys
import math


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

diff_time ={
    "Native (first)" : [[], [], [], [], [], [], [], []],
    "Native (second)" : [[], [], [], [], [], [], [], []],
    "Di3FS (bsdiff, first)" : [[], [], [], [], [], [], [], []],
    "Di3FS (bsdiff, second)" : [[], [], [], [], [], [], [], []]
}
bins = [2**4, 2**7, 2**10, 2**14, 2**17, 2**20, 2**24, 2**27]

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        task = r["taskName"]
        if task != "read":
            continue

        size = r["size"]
        if size == 0:
            continue

        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        if name != "postgres-13.1-13.2":
            continue

        enc = labels["deltaEncoding"]
        if labels["threadSchedMode"] != "none" or labels["compressionMode"] != "bzip2" or enc != "bsdiffx":
            continue

        if enc == "bsdiffx":
            enc = "bsdiff"

        if labels["count"] != "0" and labels["count"] != "1":
                continue

        cnts = ["first", "second"]
        d = ""
        if labels["pathLabel"] == "native":
            d = "Native ({})".format(cnts[int(labels["count"])])
        if labels["pathLabel"] == "di3fs":
            d = "Di3FS ({}, {})".format(enc, cnts[int(labels["count"])])

        idx = 0
        for i, bin in enumerate(bins):
            idx = i
            if bin > size:
                break
        
        diff_time[d][idx].append(r["elapsedMicroseconds"])

ticks = ["~16B", "~128B", "~1KiB", "~16KiB", "~128KiB", "~1MiB", "~16MiB", "~128MiB"]
colors = plt.rcParams['axes.prop_cycle'].by_key()['color']
plt.rcParams["figure.figsize"] = (12,5)
plt.rcParams["font.size"] = 18
plt.ylabel("Elapsed time (Microseconds)")
plt.xlabel("File size")
plt.yscale('log')
bps = []
for i, k in enumerate(sorted(diff_time.keys())):
    bp = plt.boxplot(diff_time[k], widths=[BAR_WIDTH for x in range(0, 8)], positions=[((x*4.5) + i)*BAR_WIDTH + (BAR_WIDTH/2) for x in range(0, 8)], sym="", patch_artist=True)
    for median in bp['medians']:
        median.set_color('black')
    for b in bp['boxes']:
        b.set_facecolor(colors[i])
    bps.append((bp, k))
plt.xticks([(x*4.5 + 2)*BAR_WIDTH for x in range(0, 8)], ticks)
plt.xlim(0, (7*4.5 + 4)*BAR_WIDTH)
plt.legend([bps[i][0]["boxes"][0] for i in range(0, len(bps))], [bps[i][1] for i in range(0, len(bps))], loc="upper left")
plt.tight_layout()
plt.savefig("eval-file-io-read-boxplot.pdf")
