import time

start = time.time()

import sys
label = sys.argv[1]
import torch
from torchvision.models import efficientnet_b0
from pytorch_benchmark import benchmark

load_finish = time.time()

model = efficientnet_b0().to("cpu") 
sample = torch.randn(1, 3, 224, 224)  # (B, C, H, W)
results = benchmark(model, sample, num_runs=1000)
result = results["timing"]["batch_size_1"]["on_device_inference"]["metrics"]
run_finish = time.time()
print({label: {"bench":result, "start": start, "load_finish": load_finish, "run_finish": run_finish}})
