import matplotlib.pyplot as plt
import sys
import json
import re

diff_time = {}
diff_mem = {}
diff_ratio = {}
with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        target_file = r["target"].split(" ")[0]
        if target_file not in diff_time:
            diff_time[target_file] = {}
            diff_mem[target_file] = {}
            diff_ratio[target_file] = {}
        
        result = re.sub('\s+', ' ', r["result"])
        avg = result.split(" ")[1]
        diff_time[target_file][r["algorithm"]] = float(avg)
        diff_mem[target_file][r["algorithm"]] = float(r["maxMemUse(KB)"] * 1000) / 1024.0 / 1024.0
        diff_ratio[target_file][r["algorithm"]] = float(r["size"]) / float(r["newSize"]) * 100.0

print(diff_time)
print(diff_ratio)

BAR_WIDTH=0.2

plt.style.use("seaborn-v0_8-deep")

plt.rcParams["figure.figsize"] = (10,4)
plt.rcParams["font.size"] = 13
fig, ax = plt.subplots(nrows=1, ncols=3, sharex=True)
ax[0].set_ylabel("Generation time (seconds)", fontsize = 15)
ax[1].set_ylabel("Maximum memory use (MiB)", fontsize = 15)
ax[2].set_ylabel("Delta size ratio (%)", fontsize = 15)

labels = ["xdelta3", "bsdiffx"]
keys = ["postgres", "redis"]
keys_show = ["postgres\n13.1-13.2\n(bin, 7.6MiB)", "redis\n7.0.5-7.0.6\n(tar, 111.4MiB)"]
cycle = plt.rcParams['axes.prop_cycle'].by_key()['color']

data_num = len(labels)
factor = (data_num+1) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(diff_time[k][l])
    l = l.replace("bsdiffx", "bsdiff", -1)
    ax[0].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l, color=cycle[i+1])
ax[0].legend(fontsize=14)
#ax[0].set_yscale("log")

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(diff_mem[k][l])
    l = l.replace("bsdiffx", "bsdiff", -1)
    ax[1].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l, color=cycle[i+1])
#ax[1].legend(fontsize=14)

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        value.append(diff_ratio[k][l])
    l = l.replace("bsdiffx", "bsdiff", -1)
    ax[2].bar([x*factor+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l, color=cycle[i+1])
#ax[2].legend(fontsize=14)

plt.xlim(0, (len(keys)-1)*factor+BAR_WIDTH*data_num)
plt.xticks([x*factor+BAR_WIDTH*data_num/2 for x in range(0, len(keys_show))], [x for x in keys_show])
plt.tight_layout()

plt.savefig("compare-deltas.pdf")