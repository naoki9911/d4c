import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

diff_time ={}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        task = r["taskName"]
        if task != "open+read":
            continue
        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        if name != "postgres-13.1-13.2":
            continue

        if labels["threadSchedMode"] != "none" or labels["compressionMode"] != "bzip2":
            continue

        if labels["count"] != "0" and labels["count"] != "1":
                continue
        d = "{} ({}, cnt={})".format(labels["pathLabel"], labels["deltaEncoding"], labels["count"])
        if labels["pathLabel"] == "native":
            d = "native (cnt={})".format(labels["count"])

        if d not in diff_time:
            diff_time[d] = ([], [])
        
        diff_time[d][0].append(r["size"])
        diff_time[d][1].append(r["elapsedMicroseconds"])

print(diff_time.keys())

plt.rcParams["figure.figsize"] = (12,6)
plt.rcParams["font.size"] = 18
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Elapsed time (Microseconds)")
ax.set_xlabel("File size (bytes)")
plt.yscale('log')
plt.xscale('log')
for l in sorted(diff_time.keys()):
    ax.scatter(diff_time[l][0], diff_time[l][1], marker="+", label=l, alpha=0.8)

plt.legend()
plt.tight_layout()

plt.savefig("eval-file-io-open-read.pdf")
