import matplotlib.pyplot as plt
import numpy as np 
import json
import sys
import csv


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

storage_usage ={}

with open(sys.argv[1]) as csvfile:
    reader = csv.reader(csvfile)
    l = [r for r in reader]
    header = l[0]
    if not(header[0] == "images" and header[1] == "native" and header[2] == "xdelta3" and header[3] == "bsdiffx"):
        raise Exception("invalid format")

    for l in l[1:]:
        storage_usage[l[0]] = {}
        if l[0] == "pytorch":
            storage_usage[l[0]][header[1]] = float(l[1]) / 1024
            storage_usage[l[0]][header[2]] = float(l[2]) / 1024
            storage_usage[l[0]][header[3]] = float(l[3]) / 1024
        else:
            storage_usage[l[0]][header[1]] = float(l[1])
            storage_usage[l[0]][header[2]] = float(l[2])
            storage_usage[l[0]][header[3]] = float(l[3])


plt.rcParams["figure.figsize"] = (6,4)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax2 = ax.twinx()
ax.set_ylabel("Storage usage (MiB)")
ax2.set_ylabel("Storage usage (GiB, pytorch)")
#ax.set_title("diff_size")

labels = [("native", "native"), ("xdelta3", "xdelta3"), ("bsdiffx", "bsdiff")]
keys = list(storage_usage.keys())

data_num = len(labels)
factor = (data_num+0.5) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys[0:len(keys)-1]:
        value.append(storage_usage[k][l[0]])
    ax.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(0, len(keys)-1)], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])

for i, l in zip(range(0, data_num), labels):
    value = []
    value.append(storage_usage["pytorch"][l[0]])
    ax2.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in [len(keys)-1]], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])

ax.legend(loc=9)
ax.tick_params()
plt.xlim(0, (len(keys)-1)*factor+ (int((len(keys)-1)/3) * BAR_WIDTH/3)+BAR_WIDTH*data_num)
plt.xticks([x*factor+ (int(x/3)*BAR_WIDTH/3) + BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x for x in keys])
plt.tight_layout()
ax2.text(2.57, 0.1, "N/A", fontsize=12)

plt.savefig("eval-storage-usage.pdf")