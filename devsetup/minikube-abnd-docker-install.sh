#!/usr/bin/env bash
pushd .
mkdir ./tempinstall
cd ./tempinstall
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube_latest_amd64.deb
sudo dpkg -i minikube_latest_amd64.deb
sudo apt install docker.io 
sudo apt install podman-docker  # version 4.9.3+ds1-1ubuntu0.2^C
rm -rf ./tempinstall
popd
