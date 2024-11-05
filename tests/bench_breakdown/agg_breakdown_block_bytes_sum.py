import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_results = {}
merge_results["AddAndAdd"] = ([], [], [], [])
merge_results["AddAndInsert"] = ([], [], [], [])
merge_results["InsertAndAdd"] = ([], [], [], [])
merge_results["InsertAndInsert"] = ([], [], [], [])

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "delta-merging":
            continue

        labels = r["labels"]
        fileSize = r["size"]
        elapsedMicroseconds = r["elapsedMicroseconds"]

        AddAndAdd = int(labels["ms.AddAndAdd.Bytes"])
        AddAndInsert = int(labels["ms.AddAndInsert.Bytes"])
        InsertAndAdd = int(labels["ms.InsertAndAdd.Bytes"])
        InsertAndInsert = int(labels["ms.InsertAndInsert.Bytes"])
        AddAndAddMicroSec = int(labels["ms.AddAndAdd.Microseconds"])
        AddAndInsertMicroSec = int(labels["ms.AddAndInsert.Microseconds"])
        InsertAndAddMicroSec = int(labels["ms.InsertAndAdd.Microseconds"])
        InsertAndInsertMicroSec = int(labels["ms.InsertAndInsert.Microseconds"])

        if AddAndAdd != 0 and AddAndAddMicroSec > 100:
            merge_results["AddAndAdd"][0].append(fileSize)
            merge_results["AddAndAdd"][1].append(AddAndAdd)
            merge_results["AddAndAdd"][2].append(AddAndAddMicroSec)
            merge_results["AddAndAdd"][3].append(AddAndAdd/AddAndAddMicroSec)

        if AddAndInsert != 0 and AddAndInsertMicroSec > 100:
            merge_results["AddAndInsert"][0].append(fileSize)
            merge_results["AddAndInsert"][1].append(AddAndInsert)
            merge_results["AddAndInsert"][2].append(AddAndInsertMicroSec)
            merge_results["AddAndInsert"][3].append(AddAndInsert/AddAndInsertMicroSec)

        if InsertAndAdd != 0 and InsertAndAddMicroSec > 100:
            merge_results["InsertAndAdd"][0].append(fileSize)
            merge_results["InsertAndAdd"][1].append(InsertAndAdd)
            merge_results["InsertAndAdd"][2].append(InsertAndAddMicroSec)
            merge_results["InsertAndAdd"][3].append(InsertAndAdd/InsertAndAddMicroSec)

        if InsertAndInsert != 0 and InsertAndInsertMicroSec > 100:
            merge_results["InsertAndInsert"][0].append(fileSize)
            merge_results["InsertAndInsert"][1].append(InsertAndInsert)
            merge_results["InsertAndInsert"][2].append(InsertAndInsertMicroSec)
            merge_results["InsertAndInsert"][3].append(InsertAndInsert/InsertAndInsertMicroSec)

        #merge_results["All"][0].append(fileSize)
        #merge_results["All"][1].append(AddAndAdd+AddAndInsert+InsertAndAdd+InsertAndInsert)
        #merge_results["All"][2].append(elapsedMicroseconds)

plt.rcParams["figure.figsize"] = (12,6)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Elapsed time (Microseconds)")
ax.set_xlabel("Bytes")
plt.yscale('log')
plt.xscale('log')
for l in sorted(merge_results.keys()):
    ax.scatter(merge_results[l][1], merge_results[l][3], marker="+", label=l, alpha=0.8)

plt.legend()
plt.tight_layout()

plt.savefig("breakdown-blocks-bytes-sum.pdf")