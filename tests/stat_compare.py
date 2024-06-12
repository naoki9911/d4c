import json
import sys

compare = {}

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        labels = r["labels"]
        imageName = labels["imageName"]
        new = labels["new"]
        old = labels["old"]
        path = r["path"]
        fileSize = r["fileSize"]
        # ignore 0-bytes file
        # file maybe FILE_SAME, it must be ignored
        fileDiffSize = r["fileEntryACompressionSize"]
        binaryDiffSize = r["fileEntryBCompressionSize"]
        if fileSize == 0 or binaryDiffSize == 0 or fileDiffSize == 0:
            continue
        efficiency = float(binaryDiffSize) / float(fileDiffSize)
        tag = "{}:{}-{}".format(imageName, old, new)
        if tag not in compare:
            # [path, fileSize, efficiency]
            compare[tag] = []
        compare[tag].append((path, fileSize, efficiency))

for c in compare:
    stat = compare[c]
    res = sorted(stat, key=lambda x: x[1], reverse=True)
    for r in res[0:10]:
        print(r)