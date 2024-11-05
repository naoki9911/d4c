import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_results = {}
merge_results["AddAndAdd"] = ([], [], [])
merge_results["AddAndInsert"] = ([], [], [])
merge_results["InsertAndAdd"] = ([], [], [])
merge_results["InsertAndInsert"] = ([], [], [])
merge_results["All"] = ([], [], [])

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "delta-merging":
            continue

        labels = r["labels"]
        fileSize = r["size"]
        elapsedMicroseconds = r["elapsedMicroseconds"]

        AddAndAdd = int(labels["ms.AddAndAdd.Count"])
        AddAndInsert = int(labels["ms.AddAndInsert.Count"])
        InsertAndAdd = int(labels["ms.InsertAndAdd.Count"])
        InsertAndInsert = int(labels["ms.InsertAndInsert.Count"])

        AddAndAddMicroSec = int(labels["ms.AddAndAdd.Microseconds"])
        AddAndInsertMicroSec = int(labels["ms.AddAndInsert.Microseconds"])
        InsertAndAddMicroSec = int(labels["ms.InsertAndAdd.Microseconds"])
        InsertAndInsertMicroSec = int(labels["ms.InsertAndInsert.Microseconds"])

        merge_results["AddAndAdd"][0].append(fileSize)
        merge_results["AddAndAdd"][1].append(AddAndAdd)
        merge_results["AddAndAdd"][2].append(AddAndAddMicroSec)

        merge_results["AddAndInsert"][0].append(fileSize)
        merge_results["AddAndInsert"][1].append(AddAndInsert)
        merge_results["AddAndInsert"][2].append(AddAndInsertMicroSec)

        merge_results["InsertAndAdd"][0].append(fileSize)
        merge_results["InsertAndAdd"][1].append(InsertAndAdd)
        merge_results["InsertAndAdd"][2].append(InsertAndAddMicroSec)

        merge_results["InsertAndInsert"][0].append(fileSize)
        merge_results["InsertAndInsert"][1].append(InsertAndInsert)
        merge_results["InsertAndInsert"][2].append(InsertAndInsertMicroSec)

        merge_results["All"][0].append(fileSize)
        merge_results["All"][1].append(AddAndAdd+AddAndInsert+InsertAndAdd+InsertAndInsert)
        merge_results["All"][2].append(elapsedMicroseconds)

plt.rcParams["figure.figsize"] = (12,6)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=2, ncols=1, sharex=True)
#ax.set_ylabel("Elapsed time (Microseconds)")
#ax.set_xlabel("Blocks")
ax[0].set_yscale('log')
ax[0].set_xscale('log')
ax[1].set_yscale('log')
ax[1].set_xscale('log')
for l in ["All"]:#sorted(merge_results.keys()):
    ax[0].scatter(merge_results[l][0], merge_results[l][1], marker="+", label=l, alpha=0.8)
    ax[1].scatter(merge_results[l][1], merge_results[l][2], marker="+", label=l, alpha=0.8)

plt.legend()
plt.tight_layout()

plt.savefig("breakdown-blocks-count-sum.pdf")