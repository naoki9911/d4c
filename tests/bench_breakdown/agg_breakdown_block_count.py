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
merge_results["Total"] = ([], [], [])

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

        merge_results["AddAndAdd"][0].append(fileSize)
        merge_results["AddAndAdd"][1].append(AddAndAdd)

        merge_results["AddAndInsert"][0].append(fileSize)
        merge_results["AddAndInsert"][1].append(AddAndInsert)

        merge_results["InsertAndAdd"][0].append(fileSize)
        merge_results["InsertAndAdd"][1].append(InsertAndAdd)

        merge_results["InsertAndInsert"][0].append(fileSize)
        merge_results["InsertAndInsert"][1].append(InsertAndInsert)

        merge_results["Total"][0].append(fileSize)
        merge_results["Total"][1].append(AddAndAdd+AddAndInsert+InsertAndAdd+InsertAndInsert)
        merge_results["Total"][2].append(fileSize/elapsedMicroseconds)

plt.rcParams["figure.figsize"] = (12,6)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=2, ncols=1, sharex=True)
ax[0].set_ylabel("Merge speed (bytes/us)")
ax[0].set_xlabel("Number of merge operation")
ax[0].set_yscale('log')
ax[0].set_xscale('log')
ax[1].set_yscale('log')
ax[1].set_xscale('log')
ax[1].set_ylabel("Number of merge operation")
ax[1].set_xlabel("File size (bytes)")
for l in ["Total"]:#sorted(merge_results.keys()):
    ax[0].scatter(merge_results[l][1], merge_results[l][2], marker="+", label=l, alpha=0.8)
    ax[1].scatter(merge_results[l][0], merge_results[l][1], marker="+", label=l, alpha=0.8)

plt.legend()
plt.tight_layout()

plt.savefig("breakdown-blocks-count.pdf")