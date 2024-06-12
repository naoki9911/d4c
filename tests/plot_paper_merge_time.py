import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_time ={}
merge_size = {}
diff_size = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] == "merge":
            labels = r["labels"]
            sizeMiB = r["size"] / 1024 / 1024
            name = "{}-{}".format(labels["imageName"], labels["out"])
            valueName = "th-{}-sched-{}-comp-{}-enc-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"], labels["deltaEncoding"])
            if name not in merge_time:
                merge_time[name] = {}
                merge_size[name] = {}
            if valueName not in merge_time[name]:
                merge_time[name][valueName] = []
                merge_size[name][valueName] = []
            merge_time[name][valueName].append(r["elapsedMilliseconds"] / 1000)
            merge_size[name][valueName].append(sizeMiB)
        if r["taskName"] == "diff":
            labels = r["labels"]
            name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
            sizeMiB = r["size"] / 1024 / 1024
            if labels["mode"] == "binary-diff":
                valueName = "th-{}-sched-{}-comp-{}-enc-{}".format(labels["threadNum"], labels["threadSchedMode"], labels["compressionMode"], labels["deltaEncoding"])
                if name not in diff_size:
                    diff_size[name] = {}
                if valueName not in diff_size[name]:
                    diff_size[name][valueName] = []
                diff_size[name][valueName].append(sizeMiB)

labels = []
for th in ["8"]:
    for sched in ["none"]:
        for comp in ["bzip2"]:
            for enc in [("xdelta3", "xdelta3"), ("bsdiffx", "bsdiff")]:
                labels.append((("th-{}-sched-{}-comp-{}-enc-{}".format(th, sched, comp, enc[0])), enc[1]))

plt.rcParams["figure.figsize"] = (9,4)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=2, sharex=True)
ax[0].set_ylabel("Time to generate (Seconds)")
ax[1].set_ylabel("Delta bundle size ratio\n(merge/diff)")
#ax.set_title("diff_size")

keys = list(merge_time.keys())
keys = []
keys.append(("postgres-13.1-13.3", "postgres\n.1→.3"))
keys.append(("nginx-1.23.1-1.23.3", "nginx\n.1→.3"))
keys.append(("redis-7.0.5-7.0.7", "redis\n.5→.7"))
keys.append(("redis-7.0.5-7.0.7", "redis\n.5→.7"))

cycle = plt.rcParams['axes.prop_cycle'].by_key()['color']

data_num = len(labels)
factor = (data_num+0.5) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        v = merge_time[k[0]][l[0]]
        value.append(sum(v) / len(v))
    ax[0].bar([(x*factor + (int(x/1)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1], color=cycle[i+1])

for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys:
        mv = merge_size[k[0]][l[0]]
        mv_avg = sum(mv) / len(mv)
        dv = diff_size[k[0]][l[0]]
        dv_avg = sum(dv) / len(dv)
        value.append(mv_avg/dv_avg)
    ax[1].bar([(x*factor + (int(x/1)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(0, len(keys))], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1], color=cycle[i+1])

ax[0].legend()
ax[0].tick_params()
plt.xlim(0, (len(keys)-1)*factor+ (int((len(keys)-1)/1) * BAR_WIDTH/3)+BAR_WIDTH*data_num)
plt.xticks([x*factor+ (int(x/1)*BAR_WIDTH/3) + BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x[1] for x in keys])
plt.tight_layout()

plt.savefig("eval-merge-time.pdf")