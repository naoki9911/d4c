import json
import sys


files = {}
comp_eff = []

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l)
        labels = r["labels"]
        if labels["deltaEncoding"] != "bsdiffx":
            continue
        if labels["imageName"] != "postgres":
            continue
        if labels["old"] != "13.1":
            continue
        if labels["new"] != "13.2":
            continue
        if r["fileEntryBType"] != 2: # only handle FILE_DIFF
            continue

        path = r["path"]
        if path in files:
            continue
        
        files[path] = None

        compSize = r["fileEntryACompressionSize"]
        diffSize = r["fileEntryBCompressionSize"]
        comp_eff.append({
            "path": r["path"],
            "compSize": compSize,
            "diffSize": diffSize,
            "reductionSize": compSize - diffSize,
            "ratio": diffSize / compSize,
        })

print("THE WORST REDUCED FILES")
print("path, reducedSize(bytes), ratio")
comp_eff = sorted(comp_eff, key=lambda x: x["reductionSize"])
for l in comp_eff[:20]:
    print("{},{},{}".format(l["path"], l["reductionSize"], l["ratio"]))

print("THE MOST REDUCED FILES")
comp_eff = sorted(comp_eff, key=lambda x: x["reductionSize"], reverse=True)
for l in comp_eff[:20]:
    print("{},{},{}".format(l["path"], l["reductionSize"], l["ratio"]))