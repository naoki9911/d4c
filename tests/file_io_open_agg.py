import numpy as np 
import json
import sys

stats = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        task = r["taskName"]
        if task != "di3fs-open":
            continue
        labels = r["labels"]

        enc = labels["deltaEncoding"]
        if enc != "bsdiffx":
            continue

        elapsedSeconds = r["elapsedMicroseconds"] / 1000 / 1000
        readBytes = int(labels["readBytesFromImage"])
        sizePerBytes = readBytes / elapsedSeconds
        stats[sizePerBytes] = (readBytes, labels["fileEntryType"], int(labels["fileSize"]), r["elapsedMicroseconds"], labels["path"])

stats = sorted(stats.items(), key=lambda x:x[0], reverse=True)
for s in stats[0:10]:
    print(s)
