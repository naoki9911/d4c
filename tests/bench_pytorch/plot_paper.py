import matplotlib.pyplot as plt
import numpy as np 
import json
import sys


BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")


def aggregate(raw_data):
    res = {}
    for k in raw_data:
        res[k] = {"avg": [], "stdev": []}
        for l in raw_data[k]:
            res[k]["avg"].append(np.mean(l))
            res[k]["stdev"].append(np.std(l))
    return res

def aggregate_sqrt(raw_data):
    res = {}
    for k in raw_data:
        res[k] = {"avg": []}
        for l in raw_data[k]:
            res[k]["avg"].append(np.sqrt(np.mean((np.array(l) ** 2))))
    return res

def init_dict():
    res = {}
    res["Di3FS(bsdiff)"] = []
    res["Native"] = []
    for i in range(0, 5):
        res["Di3FS(bsdiff)"].append([])
        res["Native"].append([])
        for _ in range(0, 10):
            res["Di3FS(bsdiff)"][i].append(0)
            res["Native"][i].append(0)
    return res

bench_results=init_dict()
bench_std_results=init_dict()
load_results=init_dict()
run_results=init_dict()

with open(sys.argv[1]) as f:
    for l in f.readlines():
        r = json.loads(l.replace("'", '"'))
        k = list(r.keys())[0]
        cols = k.split("-")
        if len(cols) != 9:
            continue
        fs = cols[0]
        if fs == "di3fs":
            fs = "Di3FS(bsdiff)"
        elif fs == "native":
            fs = "Native"
        else:
            continue
        epoch = int(cols[6])
        run = int(cols[8])

        #print("{}-{}-{}".format(fs, epoch, run))
        seconds_per_batch = r[k]["bench"]["seconds_per_batch_mean"] * 1000
        seconds_per_batch_std = r[k]["bench"]["seconds_per_batch_std"] * 1000
        bench_results[fs][run][epoch] = seconds_per_batch
        bench_std_results[fs][run][epoch] = seconds_per_batch_std

        start = r[k]["start"]
        load_finish = r[k]["load_finish"]
        run_finish = r[k]["run_finish"]
        load_results[fs][run][epoch] = load_finish - start
        run_results[fs][run][epoch] = run_finish - load_finish


#print(bench_results)
#print(load_results)
#print(run_results)

bench_agg = aggregate(bench_results)
bench_std_agg = aggregate_sqrt(bench_std_results)
load_agg = aggregate(load_results)
run_agg = aggregate(run_results)

#print(bench_agg)
#print(load_agg)
#print(run_agg)

run = [1, 2, 3, 4, 5]
plt.rcParams["figure.figsize"] = (10,5)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=2, sharex=True)
ax[0].set_ylabel("Average inference time (Milliseconds)")
ax[0].set_xlabel("N-th round in benchmark loop")
for l in bench_agg:
    ax[0].errorbar(run, bench_agg[l]["avg"], yerr=bench_std_agg[l]["avg"], capsize=5, marker="o", linestyle="dashed", label=l)
    #ax.label(p)
ax[0].legend()
ax[0].set_ylim(0, 40)

#ax12 = ax[1].twinx()
ax[1].set_ylabel("Loading libraries time (seconds)")
ax[1].set_xlabel("N-th round in benchmark loop")
#ax12.set_ylabel("Running benchmark time (seconds)")
for l in load_agg:
    ax[1].errorbar(run, load_agg[l]["avg"], yerr=load_agg[l]["stdev"], capsize=5, marker="o", linestyle="dotted", label="Load libs({})".format(l).replace("(bsdiff)", ""))
#for l in run_agg:
#    ax12.errorbar(run, run_agg[l]["avg"], yerr=run_agg[l]["stdev"], capsize=5, marker="o", linestyle="solid", label="Run bench({})".format(l).replace("(bsdiff)", ""))

ax[1].legend()
#h1, l1 = ax[1].get_legend_handles_labels()
#h2, l2 = ax12.get_legend_handles_labels()
#ax[1].legend(h1+h2, l1+l2, loc='center right')

plt.tight_layout()

plt.savefig("eval-pytorch.pdf")
