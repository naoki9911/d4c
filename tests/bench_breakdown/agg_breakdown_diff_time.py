import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_results = {}
merge_results["entire"] = ([], [])
merge_results["diff"] = ([], [])
merge_results["write"] = ([], [])

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "diff":
            continue

        labels = r["labels"]
        fileSize = r["size"]
        elapsedMicroseconds = r["elapsedMicroseconds"]
        diffMicroSec = int(labels["diffMircoseconds"])
        writeMicroSec = int(labels["writeMicroseconds"])

        merge_results["entire"][0].append(fileSize)
        merge_results["entire"][1].append(elapsedMicroseconds)

        merge_results["diff"][0].append(fileSize)
        merge_results["diff"][1].append(diffMicroSec)

        merge_results["write"][0].append(fileSize)
        merge_results["write"][1].append(writeMicroSec)

plt.rcParams["figure.figsize"] = (12,6)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Elapsed time (Microseconds)")
ax.set_xlabel("File size (bytes)")
plt.yscale('log')
plt.xscale('log')
for l in sorted(merge_results.keys()):
    ax.scatter(merge_results[l][0], merge_results[l][1], marker="+", label=l, alpha=0.8)

plt.legend()
plt.tight_layout()

plt.savefig("breakdown-diff-time.pdf")