import matplotlib.pyplot as plt
import numpy as np 
import json
import sys
from matplotlib.ticker import ScalarFormatter


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_results = {}
ops = ["AddAndAdd", "AddAndInsert", "InsertAndAdd", "InsertAndInsert"]
for op in ops:
    merge_results[op] = ([], [], [], [], [])
merge_results["Total"] = ([], [], [], [], [])

sizeBoxplot = ([], [], [], [], [], [])
sizeBins = [2**10, 2**14, 2**17, 2**20, 2**24, 2**27]
opsBoxplot = ([], [], [], [], [])
opsBins = [10**1, 10**2, 10**3, 10**4, 10**10]

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "delta-merging":
            continue

        labels = r["labels"]
        fileSize = r["size"]
        elapsedMicroseconds = r["elapsedMicroseconds"]

        AddAndAddCount = int(labels["ms.AddAndAdd.Count"])
        AddAndInsertCount = int(labels["ms.AddAndInsert.Count"])
        InsertAndAddCount = int(labels["ms.InsertAndAdd.Count"])
        InsertAndInsertCount = int(labels["ms.InsertAndInsert.Count"])
        AddAndAddBytes = int(labels["ms.AddAndAdd.Bytes"])
        AddAndInsertBytes = int(labels["ms.AddAndInsert.Bytes"])
        InsertAndAddBytes = int(labels["ms.InsertAndAdd.Bytes"])
        InsertAndInsertBytes = int(labels["ms.InsertAndInsert.Bytes"])
        AddAndAddMicroSec = int(labels["ms.AddAndAdd.Microseconds"])
        AddAndInsertMicroSec = int(labels["ms.AddAndInsert.Microseconds"])
        InsertAndAddMicroSec = int(labels["ms.InsertAndAdd.Microseconds"])
        InsertAndInsertMicroSec = int(labels["ms.InsertAndInsert.Microseconds"])

        merge_results["AddAndAdd"][0].append(fileSize)
        merge_results["AddAndAdd"][1].append(AddAndAddCount)
        merge_results["AddAndAdd"][2].append(AddAndAddBytes)
        if AddAndAddMicroSec > 100:
            merge_results["AddAndAdd"][3].append(AddAndAddBytes/AddAndAddMicroSec)
            merge_results["AddAndAdd"][4].append(AddAndAddBytes/AddAndAddCount)

        merge_results["AddAndInsert"][0].append(fileSize)
        merge_results["AddAndInsert"][1].append(AddAndInsertCount)
        merge_results["AddAndInsert"][2].append(AddAndInsertBytes)
        if AddAndInsertMicroSec > 100:
            merge_results["AddAndInsert"][3].append(AddAndInsertBytes/AddAndInsertMicroSec)
            merge_results["AddAndInsert"][4].append(AddAndInsertBytes/AddAndInsertCount)

        merge_results["InsertAndAdd"][0].append(fileSize)
        merge_results["InsertAndAdd"][1].append(InsertAndAddCount)
        merge_results["InsertAndAdd"][2].append(InsertAndAddBytes)
        if InsertAndAddMicroSec > 100:
            merge_results["InsertAndAdd"][3].append(InsertAndAddBytes/InsertAndAddMicroSec)
            merge_results["InsertAndAdd"][4].append(InsertAndAddBytes/InsertAndAddCount)

        merge_results["InsertAndInsert"][0].append(fileSize)
        merge_results["InsertAndInsert"][1].append(InsertAndInsertCount)
        merge_results["InsertAndInsert"][2].append(InsertAndInsertBytes)
        if InsertAndInsertMicroSec > 100:
            merge_results["InsertAndInsert"][3].append(InsertAndInsertBytes/InsertAndInsertMicroSec)
            merge_results["InsertAndInsert"][4].append(InsertAndInsertBytes/InsertAndInsertCount)

        idx = 0
        for i, bin in enumerate(sizeBins):
            idx = i
            if bin > fileSize:
                break
        sizeBoxplot[idx].append(elapsedMicroseconds)

        opsCount = AddAndAddCount+AddAndInsertCount+InsertAndAddCount+InsertAndInsertCount
        idx = 0
        for i, bin in enumerate(opsBins):
            idx = i
            if bin > opsCount:
                break
        opsBoxplot[idx].append(fileSize/elapsedMicroseconds)
        merge_results["Total"][0].append(fileSize)
        merge_results["Total"][1].append(elapsedMicroseconds)
        if labels["count"] == "2":
            merge_results["Total"][2].append(AddAndAddCount+AddAndInsertCount+InsertAndAddCount+InsertAndInsertCount)
            merge_results["Total"][3].append(fileSize/elapsedMicroseconds)

plt.rcParams["figure.figsize"] = (12,5)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=2, sharex=False)

sizeTicks = ["~1KiB", "~16KiB", "~128KiB", "~1MiB", "~16MiB", "~128MiB"]
ax[0].set_xlabel("Merged file size (bytes)")
ax[0].set_ylabel("Elapsed time for DeltaMerging (us)")
ax[0].set_yscale('log')
bp = ax[0].boxplot(sizeBoxplot, widths=[BAR_WIDTH for x in range(0, len(sizeTicks))], positions=[x*(BAR_WIDTH * 1.5) + (BAR_WIDTH/2) for x in range(0, len(sizeTicks))], sym="", patch_artist=True)
for median in bp['medians']:
    median.set_color('black')
ax[0].set_xticks([x*(BAR_WIDTH *1.5)+ BAR_WIDTH/2 for x in range(0, len(sizeTicks))], sizeTicks, fontsize=13)
ax[0].set_xlim(0, len(sizeTicks)*BAR_WIDTH*1.5 - BAR_WIDTH/2)

opsTicks = ["~$10^1$", "~$10^2$", "~$10^3$", "~$10^4$", "$10^4$~"]
ax[1].set_ylabel("DeltaMerging speed (bytes/us)")
ax[1].set_xlabel("Number of merge block operations in a file")
#bp = ax[1].boxplot(opsBoxplot, widths=[BAR_WIDTH for x in range(0, len(opsTicks))], positions=[x*(BAR_WIDTH * 1.5) + (BAR_WIDTH/2) for x in range(0, len(opsTicks))], sym="", patch_artist=True)
#for median in bp['medians']:
#    median.set_color('black')
#ax[1].set_xticks([x*(BAR_WIDTH *1.5)+ BAR_WIDTH/2 for x in range(0, len(opsTicks))], opsTicks, fontsize=13)
#ax[1].set_xlim(0, len(opsTicks)*BAR_WIDTH*1.5 - BAR_WIDTH/2)
ax[1].set_yscale('log')
ax[1].set_xscale('log')
ax[1].scatter(merge_results["Total"][2], merge_results["Total"][3], marker="+", label=l, alpha=0.8)

plt.tight_layout()
plt.savefig("merge-breakdown.pdf")
plt.clf()


plt.rcParams["figure.figsize"] = (12,4.85)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=2, sharex=False)

ops_count = []
ops_bytes = []
for op in ops:
    ops_count.append(sum(merge_results[op][1]) / 10)
    ops_bytes.append(sum(merge_results[op][2]) / 10)

print(ops_count)
print(ops_bytes)

colors = plt.rcParams['axes.prop_cycle'].by_key()['color']
factor = BAR_WIDTH*2.4
ax[0].yaxis.set_major_formatter(ScalarFormatter(useMathText=True))
ax[0].ticklabel_format(style="sci",  axis="y",scilimits=(0,0))
ax[0].set_ylabel("Number of merge block operations")
p = ax[0].bar([(x*factor) for x in range(0, len(ops))], ops_count, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label="Number of ops", color=colors[0])
ax0_twin = ax[0].twinx()
ax0_twin.set_yscale('log')
ax0_twin.set_ylabel("Processed bytes")
ax0_twin.bar([(x*factor) + BAR_WIDTH for x in range(0, len(ops))], ops_bytes, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label="Merged bytes", color=colors[1])

handler1, label1 = ax[0].get_legend_handles_labels()
handler2, label2 = ax0_twin.get_legend_handles_labels()
ax[0].legend(handler1 + handler2, label1 + label2)
ax[0].set_xlim(0, factor*(len(ops)-1) + BAR_WIDTH*2)
ax[0].set_xticks([(x*factor) + BAR_WIDTH for x in range(0, len(ops))], ["ADD\n+\nADD", "ADD\n+\nINSERT", "INSERT\n+\nADD", "INSERT\n+\nINSERT"], fontsize=16)


ops_speed_mean = []
ops_speed_stdev = []
ops_bytes_mean = []
ops_bytes_stdev = []
for op in ops:
    ops_speed_mean.append(np.mean(merge_results[op][3]))
    ops_speed_stdev.append(np.std(merge_results[op][3]))
    ops_bytes_mean.append(np.mean(merge_results[op][4]))
    ops_bytes_stdev.append(np.std(merge_results[op][4]))

factor = BAR_WIDTH*1.6
p = ax[1].bar([(x*factor) for x in range(0, len(ops))], ops_speed_mean, yerr=ops_speed_stdev, capsize=10, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label="Number of ops", color=colors[0])
ax[1].set_ylim(0, 500)
ax[1].set_xlim(0, factor*(len(ops)-1) + BAR_WIDTH)
ax[1].set_xticks([(x*factor) + BAR_WIDTH/2 for x in range(0, len(ops))], ["ADD\n+\nADD", "ADD\n+\nINSERT", "INSERT\n+\nADD", "INSERT\n+\nINSERT"], fontsize=16)
ax[1].set_ylabel("Merge operation speed (bytes/us)")

plt.tight_layout()
plt.savefig("merge-breakdown-2.pdf")