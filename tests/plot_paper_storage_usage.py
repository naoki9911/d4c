import matplotlib.pyplot as plt
import numpy as np 
import sys
import os



BAR_WIDTH=0.2
plt.style.use("seaborn-v0_8-deep")

images = {
    "postgres": ["13.1", "13.2", "13.3"],
    "nginx": ["1.23.1", "1.23.2", "1.23.3"],
    "redis": ["7.0.5", "7.0.6", "7.0.7"],
    "pytorch": ["2.2.0-cuda12.1-cudnn8-runtime", "2.2.1-cuda12.1-cudnn8-runtime", "2.2.2-cuda12.1-cudnn8-runtime"],
}
# native's storage usage was acquired via 'docker system df' when each image pulled
# Each size is represented in MiB.
storage_usage ={
    "postgres":{
        "native": 899.5056152
    },
    "nginx":{
        "native": 405.5976868
    },
    "redis":{
        "native": 253.5820007
    },
    "pytorch":{
        "native": 21772.38464
    },
}

image_dir = sys.argv[1]
for image in images:
    versions = images[image]
    base_path = os.path.join(image_dir, image, versions[0]+".cdimg")
    base_size = os.path.getsize(base_path)
    for enc in ["bsdiffx", "xdelta3"]:
        lower_path = os.path.join(image_dir, image, "diff_{}-{}-{}.cdimg".format(versions[0], versions[1], enc))
        upper_path = os.path.join(image_dir, image, "diff_{}-{}-{}.cdimg".format(versions[1], versions[2], enc))

        lower_size = os.path.getsize(lower_path)
        upper_size = os.path.getsize(upper_path)
        storage_usage[image][enc] = (base_size + lower_size + upper_size) / 1024 / 1024

plt.rcParams["figure.figsize"] = (6,4)
plt.rcParams["font.size"] = 16
fig, ax = plt.subplots(nrows=1, ncols=1, sharex=True)
ax2 = ax.twinx()
ax.set_ylabel("Storage usage (MiB)")
ax2.set_ylabel("Storage usage (GiB, pytorch)")
#ax.set_title("diff_size")

labels = [("native", "native"), ("xdelta3", "xdelta3"), ("bsdiffx", "bsdiff")]
keys = list(storage_usage.keys())

data_num = len(labels)
factor = (data_num+0.5) * BAR_WIDTH
for i, l in zip(range(0, data_num), labels):
    value = []
    for k in keys[0:len(keys)-1]:
        value.append(storage_usage[k][l[0]])
    p = ax.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in range(0, len(keys)-1)], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])
    ax.bar_label(p, fmt="%.1f", fontsize=14, rotation=90, padding=2)
ax.set_ylim(0, 1150)

for i, l in zip(range(0, data_num), labels):
    value = []
    value.append(storage_usage["pytorch"][l[0]] / 1024) # represent in GiB
    p = ax2.bar([(x*factor + (int(x/3)*BAR_WIDTH/3))+(BAR_WIDTH*i) for x in [len(keys)-1]], value, align="edge",  edgecolor="black", linewidth=1, width=BAR_WIDTH, label=l[1])
    ax2.bar_label(p, fmt="%.1f", fontsize=14, rotation=90, padding=2)
ax2.set_ylim(0, 27)

ax.legend(loc=9)
ax.tick_params()
plt.xlim(0, (len(keys)-1)*factor+ (int((len(keys)-1)/3) * BAR_WIDTH/3)+BAR_WIDTH*data_num)
plt.xticks([x*factor+ (int(x/3)*BAR_WIDTH/3) + BAR_WIDTH*data_num/2 for x in range(0, len(keys))], [x for x in keys])
plt.tight_layout()
#ax2.text(2.57, 0.1, "N/A", fontsize=12)

plt.savefig("eval-storage-usage.pdf")