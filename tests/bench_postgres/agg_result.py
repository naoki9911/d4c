import numpy as np

latency_native = [9.993, 11.309, 10.637, 10.946, 11.803, 12.506, 12.317, 12.603, 11.036, 12.249]
latency_di3fs = [9.926, 11.418, 11.417, 11.810, 12.199, 11.414, 11.598, 12.114, 12.691, 10.907]
tps_native = [1000.734821, 884.260782, 940.123946, 913.658994, 847.310529, 799.663580, 811.945759, 793.520975, 906.181154, 816.457601]
tps_di3fs = [1007.539842, 875.832563, 875.922636, 846.754922, 819.740193, 876.155021, 862.261488, 825.535080, 788.009779, 916.874031]

print("Latency native={} +- {}".format(np.mean(latency_native), np.std(latency_native)))
print("Latency di3fs={} +- {}".format(np.mean(latency_di3fs), np.std(latency_di3fs)))
print("TPS native={} +- {}".format(np.mean(tps_native), np.std(tps_native)))
print("TPS di3fs={} +- {}".format(np.mean(tps_di3fs), np.std(tps_di3fs)))

# https://bellcurve.jp/statistics/course/9427.html
from scipy import stats
print(stats.ttest_ind(latency_native, latency_di3fs, equal_var=False))
print(stats.ttest_ind(tps_native, tps_di3fs, equal_var=False))
