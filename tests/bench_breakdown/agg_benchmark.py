import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

merge_results = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        if r["taskName"] != "merge-per-file":
            continue
        labels = r["labels"]
        fileSize = r["size"]
        compressedSize = int(labels["compressedSize"])
        mergeMode = labels["mergeMode"]
        elapsedMicroseconds = r["elapsedMicroseconds"]

        if mergeMode not in merge_results:
            merge_results[mergeMode] = []
        
        merge_results[mergeMode].append((elapsedMicroseconds, fileSize, compressedSize))

for k in merge_results:
    totalElapsedMilliseconds = sum(i for (i, _, _) in merge_results[k]) / 1000
    print("{}: {} {} ms".format(k, len(merge_results[k]), totalElapsedMilliseconds))