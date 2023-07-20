import csv 
import sys

logs = {}
f = open(sys.argv[1])
reader = csv.reader(f)
for row in reader:
    tag = "{},{},{},{},{}".format(row[0], row[1], row[4], row[5], row[6])
    if tag in logs:
        v = logs[tag]
        logs[tag] = (v[0] + 1, v[1] + int(row[3]))
    else:
        logs[tag] =(1, int(row[3]))

for k in logs:
    v = logs[k]
    print("{},{}".format(k, str(v[1]/v[0])))
