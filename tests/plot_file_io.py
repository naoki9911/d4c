import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.4

stat_open = {}
stat_read = {}
stat_openAndRead = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        taskName = r["taskName"]
        fileSize = r["size"]
        labels = r["labels"]
        imageName = labels["imageName"]
        new = labels["new"]
        old = labels["old"]
        pathLabel = labels["pathLabel"]
        # ignore 0-bytes file
        if fileSize == 0:
            continue
        elapsedUS = r["elapsedMicroseconds"]
        tag = "{}".format(pathLabel)
        if tag not in stat_open:
            # [fileSize, elapsed, elapsed/fileSize]
            stat_open[tag] = [[], [], []]
            stat_read[tag] = [[], [], []]
            stat_openAndRead[tag] = [[], [], []]
        if taskName == "open":
            stat_open[tag][0].append(fileSize)
            stat_open[tag][1].append(elapsedUS)
            stat_open[tag][2].append(elapsedUS/fileSize)
        if taskName == "read":
            stat_read[tag][0].append(fileSize)
            stat_read[tag][1].append(elapsedUS)
            stat_read[tag][2].append(elapsedUS/fileSize)
        if taskName == "open+read":
            stat_openAndRead[tag][0].append(fileSize)
            stat_openAndRead[tag][1].append(elapsedUS)
            stat_openAndRead[tag][2].append(elapsedUS/fileSize)


labels = []
plt.rcParams["figure.figsize"] = (20,10)
fig, ax = plt.subplots(nrows=3, ncols=2)
ax[0][0].set_ylabel("Elapsed (microseconds)")
ax[0][0].set_title("File I/O (open)")
ax[0][1].set_ylabel("Elapsed Microseconds / byte")
ax[1][0].set_ylabel("Elapsed (microseconds)")
ax[1][0].set_title("File I/O (read)")
ax[1][1].set_ylabel("Elapsed Microseconds / byte")
ax[2][0].set_ylabel("Elapsed (microseconds)")
ax[2][0].set_title("File I/O (open+read)")
ax[2][0].set_xlabel("File size (bytes)")
ax[2][1].set_ylabel("Elapsed Microseconds / byte")

for tag in stat_openAndRead:
    value = stat_open[tag]
    ax[0][0].scatter(value[0], value[1], label=tag)
    value = stat_read[tag]
    ax[1][0].scatter(value[0], value[1], label=tag)
    value = stat_openAndRead[tag]
    ax[2][0].scatter(value[0], value[1], label=tag)

agg = {
    'open': {
        'mean': [0, 0],
        'std':[0, 0]
    },
    'read': {
        'mean': [0, 0],
        'std':[0, 0]
    },
    'open+read': {
        'mean': [0, 0],
        'std':[0, 0]
    }
}

labels = ['native', 'di3fs']
agg['open']['mean'][0] = np.mean(stat_open['native'][2])
agg['open']['mean'][1] = np.mean(stat_open['di3fs'][2])
agg['open']['std'][0] = np.std(stat_open['native'][2])
agg['open']['std'][1] = np.std(stat_open['di3fs'][2])
agg['read']['mean'][0] = np.mean(stat_read['native'][2])
agg['read']['mean'][1] = np.mean(stat_read['di3fs'][2])
agg['open+read']['mean'][0] = np.mean(stat_openAndRead['native'][2])
agg['open+read']['mean'][1] = np.mean(stat_openAndRead['di3fs'][2])

ax[0][1].bar(labels, agg['open']['mean'])
ax[1][1].bar(labels, agg['read']['mean'])
ax[2][1].bar(labels, agg['open']['mean'])
ax[2][1].bar(labels, agg['read']['mean'], bottom=agg['open']['mean'])
ax[2][1].legend(['open', 'read'])

ax[0][0].legend()
ax[1][0].legend()
ax[2][0].legend()
plt.tight_layout()

plt.savefig(sys.argv[2])