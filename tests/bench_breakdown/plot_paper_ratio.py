import matplotlib.pyplot as plt
import numpy as np 
import json
import sys

BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

#labels = ["both-file-same", "upper-link", "merge", "copy-upper", "copy-lower", "apply"]
labels = ["copy-upper-metadata", "merge", "copy-upper", "copy-lower", "apply"]
images = ["postgres", "nginx", "redis", "pytorch"]

results = {}

for label in labels:
    results[label] = {}
results["total"] = {}

for image in images:
    for k in results:
        results[k][image] = []
        for i in range(0, 10):
            results[k][image].append((0, 0, 0, 0)) # (count, compressedSize, elapsed, fileSize)


with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "merge-per-file":
            continue
        lab = r["labels"]
        fileSize = r["size"]
        image = lab["image"]
        benchCount = int(lab["count"]) - 1
        compressedSize = int(lab["compressedSize"])
        mergeMode = lab["mergeMode"]
        elapsedMicroseconds = r["elapsedMicroseconds"]

        if mergeMode == "both-file-same" or mergeMode == "upper-link":
            mergeMode = "copy-upper-metadata"

        (count, csize, elapsed, fsize) = results[mergeMode][image][benchCount]
        results[mergeMode][image][benchCount] = (count+1, csize+compressedSize, elapsed+elapsedMicroseconds, fsize+fileSize)

        (count, csize, elapsed, fsize) = results["total"][image][benchCount]
        results["total"][image][benchCount] = (count+1, csize+compressedSize, elapsed+elapsedMicroseconds, fsize+fileSize)

count_agg = np.zeros((len(images), len(labels)))
csize_agg = np.zeros((len(images), len(labels)))
time_agg = np.zeros((len(images), len(labels)))
fsize_agg = np.zeros((len(images), len(labels)))

for i in range(0, len(images)):
    for j in range(0, len(labels)):
        mean = ([], [], [], [])
        for k in range(0, 10):
            mean[0].append(results[labels[j]][images[i]][k][0])
            mean[1].append(results[labels[j]][images[i]][k][1])
            mean[2].append(results[labels[j]][images[i]][k][2])
            mean[3].append(results[labels[j]][images[i]][k][3])

        count_agg[i][j] = np.mean(mean[0])
        csize_agg[i][j] = np.mean(mean[1])
        time_agg[i][j] = np.mean(mean[2])
        fsize_agg[i][j] = np.mean(mean[3])

#print(count_agg.sum(axis=1, keepdims=True))
count_agg_normalized = count_agg / count_agg.sum(axis=1, keepdims=True)
csize_agg_normalized = csize_agg / csize_agg.sum(axis=1, keepdims=True)
time_agg_normalized = time_agg / time_agg.sum(axis=1, keepdims=True)
fsize_agg_normalized = fsize_agg / fsize_agg.sum(axis=1, keepdims=True)

plt.rcParams["figure.figsize"] = (12,5)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=2, sharex=False)

label_map = {
    "both-file-same": "Copy upper (metadata)",
    "upper-link": "Copy upper (metadata)",
    "copy-upper-metadata": "Copy upper (metadata)",
    "merge": "DeltaMerging",
    "copy-upper": "Copy upper",
    "copy-lower": "Copy lower",
    "apply": "Apply&Copy"
}

factor = BAR_WIDTH*1.6
cumulative = np.zeros(len(images))
for i in range(0, len(labels)):
    value = [0, 0, 0, 0]
    p = ax[0].bar([(x*factor)  for x in range(0, len(images))], count_agg_normalized[:, i], bottom=cumulative, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=label_map[labels[i]])
    cumulative += count_agg_normalized[:, i]

#cumulative = np.zeros(len(images))
#for i in range(0, len(labels)):
#    value = [0, 0, 0, 0]
#    p = ax[1].bar([(x*factor)  for x in range(0, len(images))], csize_agg_normalized[:, i], bottom=cumulative, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=labels[i])
#    cumulative += csize_agg_normalized[:, i]

cumulative = np.zeros(len(images))
for i in range(0, len(labels)):
    value = [0, 0, 0, 0]
    p = ax[1].bar([(x*factor)  for x in range(0, len(images))], time_agg_normalized[:, i], bottom=cumulative, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=label_map[labels[i]])
    cumulative += time_agg_normalized[:, i]


#cumulative = np.zeros(len(images))
#for i in range(0, len(labels)):
#    value = [0, 0, 0, 0]
#    p = ax[3].bar([(x*factor)  for x in range(0, len(images))], fsize_agg_normalized[:, i], bottom=cumulative, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=labels[i])
#    cumulative += fsize_agg_normalized[:, i]

ax[0].set_ylim(0, 1)
ax[0].set_xlim(0, factor*(len(images)-1) + BAR_WIDTH)
ax[0].legend()
ax[0].set_ylabel("Ratio", fontsize=18)

ax[0].set_xlabel("Number of processed delta files", fontsize=18)
ax[0].set_xticks([x*factor+BAR_WIDTH/2 for x in range(0, len(images))], images, fontsize=18)

ax[1].set_ylim(0, 1)
ax[1].set_xlim(0, factor*(len(images)-1) + BAR_WIDTH)
ax[1].set_xlabel("Elapsed time to process delta files", fontsize=18)
ax[1].set_xticks([x*factor+BAR_WIDTH/2 for x in range(0, len(images))], images, fontsize=18)


plt.tight_layout()
plt.savefig("merge-ops-ratio.pdf")
