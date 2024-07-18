# To reproduce paper's benchmark
## install dependencies
```bash
sudo apt install -y build-essential jq python3-pip contained golang-go
pip3 install matplotlib
```


## build xdelta3
```bash
sudo apt install -y autoconf automake libtool
git clone https://github.com/wdhongtw/xdelta
cd xdelta/xdelta3
./generate_build_files.sh && ./configure && sudo make install
cd ../../
```

## install snapshotter plugin
```bash
sudo mkdir /etc/containerd
sudo ./install_snapshotter.sh
sudo systemctl restart containerd
```

## run benchmark
```bash
sudo ./bench.sh
sudo ./test_merge.sh
```

## run remote pull benchmark
```bash
sudo ./scp_images.sh benchmark_YYYY-mm-dd-HHMM/images REMOTE_HOST
scp ../server $REMOTE_HOST:~/d4c-server/.
scp ../plugin_bsdiffx.so $REMOTE_HOST:~/d4c-server/.
scp ../plugin_xdelta3 $REMOTE_HOST:~/d4c-server/.
```

On remote host, **install xdelta3**.
Then,
```bash
$ sudo cat /boot/config-`uname -r` | grep CONFIG_HZ=
CONFIG_HZ=250
# rate = 50 mbit
# burst = rate/CONFIG_HZ = 200kb
# limit = burst * 10 = 2000kb
$ sudo tc qdisc add dev enp2s0 root handle 1: netem delay 40ms
$ sudo tc qdisc add dev enp2s0 parent 1: handle 2: htb default 1
$ sudo tc class add dev enp2s0 parent 2: classid 2:1 htb rate 50mbit ceil 50mbit burst 200kb cburst 200kb
$ cd ~/d4c-server
$ ./server --threadNum 8
```

On the benchmark host,
```bash
sudo ./bench_pull_remote.sh <REMOTE_HOST> <REMOTE_HOST's address> benchmark_YYYY-mm-dd-HHMM/images
```

## plot figures
```bash
python3 plot_paper_diff_size.py benchmark_YYYY-mm-dd-HHMM/benchmark.log
python3 plot_paper_diff_time.py benchmark_YYYY-mm-dd-HHMM/benchmark.log
python3 plot_paper_patch_time.py benchmark_YYYY-mm-dd-HHMM/benchmark.log
python3 plot_paper_merge.py benchmark_YYYY-mm-dd-HHMM/benchmark.log
python3 plot_paper_pull_time.py benchmark-pull-remote_YYYY-mm-dd-HHMM/benchmark.log
python3 plot_paper_file_io_open_read.py benchmark_YYYY-mm-dd-HHMM/benchmark-io.log
python3 plot_paper_multithread_time.py benchmark_YYYY-mm-dd-HHMM/mt-benchmark.log
python3 plot_paper_storage_usage.py benchmark_YYYY-mm-dd-HHMM/images
python3 plot_paper_merge_many.py benchmark-merge.log
```