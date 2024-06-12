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
        if task != "diff" and task != "merge":
            continue
        labels = r["labels"]
        name = "{}-{}-{}".format(labels["imageName"], labels["old"], labels["new"])
        if name != "postgres-13.1-13.2" and name != "postgres-13.1-13.2-bsdiffx-13.2-13.3-bsdiffx" and name != "postgres-13.1-13.2-xdelta3-13.2-13.3-xdelta3":
            continue

        if labels["threadSchedMode"] != "none" or labels["compressionMode"] != "bzip2":
            continue

        d = labels["deltaEncoding"]
        if d not in diff_time:
            diff_time[d] = {}

        threadNum = int(labels["threadNum"] )
        if threadNum not in diff_time[d]:
            diff_time[d][threadNum] = {}

        if task not in diff_time[d][threadNum]:
            diff_time[d][threadNum][task] = []
        
        diff_time[d][int(labels["threadNum"])][task].append(r["elapsedMilliseconds"] / 1000)


print(diff_time)
threads = [1, 2, 4, 8]
diff_time_agg = {}

for enc in diff_time.keys():
    for th in threads:
        for task in diff_time[enc][th].keys():
            r = diff_time[enc][th][task]
            l = "{} ({})".format(task, enc.replace("bsdiffx", "bsdiff"))
            if l not in diff_time_agg:
                diff_time_agg[l] = []
            diff_time_agg[l].append(sum(r) / len(r))
print(diff_time_agg)

plt.rcParams["figure.figsize"] = (5,6)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax.set_ylabel("Elapsed time (Seconds)")
ax.set_xlabel("Number of threads")
for l in diff_time_agg:
    ax.plot(threads, diff_time_agg[l], marker="o", linestyle="dashed", label=l)

plt.legend()
plt.tight_layout()

plt.savefig("eval-multithread.pdf")
