import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_results = {}
merge_results["AddAndAdd"] = ([], [])
merge_results["AddAndInsert"] = ([], [])
merge_results["InsertAndAdd"] = ([], [])
merge_results["InsertAndInsert"] = ([], [])

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "delta-merging":
            continue

        labels = r["labels"]
        fileSize = r["size"]

        AddAndAdd = int(labels["ms.AddAndAdd.Bytes"])
        AddAndInsert = int(labels["ms.AddAndInsert.Bytes"])
        InsertAndAdd = int(labels["ms.InsertAndAdd.Bytes"])
        InsertAndInsert = int(labels["ms.InsertAndInsert.Bytes"])

        merge_results["AddAndAdd"][0].append(fileSize)
        merge_results["AddAndAdd"][1].append(AddAndAdd)

        merge_results["AddAndInsert"][0].append(fileSize)
        merge_results["AddAndInsert"][1].append(AddAndInsert)

        merge_results["InsertAndAdd"][0].append(fileSize)
        merge_results["InsertAndAdd"][1].append(InsertAndAdd)

        merge_results["InsertAndInsert"][0].append(fileSize)
        merge_results["InsertAndInsert"][1].append(InsertAndInsert)

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

plt.savefig("breakdown-blocks-bytes.pdf")