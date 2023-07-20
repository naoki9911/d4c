#!/bin/bash

./diff dimg images/nginx-1.23.1-cdimg/image.dimg images/nginx-1.23.2-cdimg/image.dimg images/base_nginx-1.23.1.dimg 1.23.1_1.23.2.dimg binary-diff
./ctr-cli pack --manifest=./images/nginx-1.23.2/manifset.json --config=./images/nginx-1.23.2/config.json --dimg=./1.23.1_1.23.2.dimg --out=./diff_nginx-1.23.1-2.cdimg
sudo ./ctr-cli load --image=d4c-nginx:1.23.1 --dimg=./images/base_nginx-1.23.1.cdimg
sudo ./ctr-cli load --image=d4c-nginx:1.23.2 --dimg=./diff_nginx-1.23.1-2.cdimg
