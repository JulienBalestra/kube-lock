#!/bin/bash

set -exuo pipefail

cd $(dirname $0)/..

kubectl delete cm lock || true

./kube-lock default lock --holder-name holder-1 --create-configmap \
    --kubeconfig-path ~/.kube/config

kubectl get cm lock -n default -o json | jq .metadata.annotations
kubectl delete cm lock -n default

set +e
./kube-lock default lock --holder-name holder-1 \
    --kubeconfig-path ~/.kube/config && exit 2

set -e
kubectl create cm lock -n default
./kube-lock default lock --holder-name holder-1 \
    --kubeconfig-path ~/.kube/config

REFERENCE=$(kubectl get cm lock -n default -o json | jq -rec .metadata.annotations)
sleep 1
./kube-lock default lock --holder-name holder-1 \
    --kubeconfig-path ~/.kube/config

CURRENT=$(kubectl get cm lock -n default -o json | jq -rec .metadata.annotations)
if [[ "${REFERENCE}" != "${CURRENT}" ]]
then
    exit 4
fi

set +e
./kube-lock default lock --holder-name holder-2 --run-once \
    --kubeconfig-path ~/.kube/config && exit 5

set -e
bash -cex "sleep 5 ;  ./kube-lock default lock --holder-name holder-1 --run-once --kubeconfig-path ~/.kube/config --unlock" &

./kube-lock default lock --holder-name holder-2 \
    --polling-interval 2s --polling-timeout 15s \
    --kubeconfig-path ~/.kube/config

wait

kubectl delete cm lock -n default
kubectl create cm lock -n default

for i in {1..3}
do
    ./kube-lock default lock --holder-name holder-${i} --create-configmap --max-holders 3 \
        --polling-interval 2s --polling-timeout 15s \
        --reason "reason ${i}" \
        --kubeconfig-path ~/.kube/config
done

kubectl get cm lock -n default -o json | jq -r '.metadata.annotations["kube-lock"]' | jq -re .


for i in {1..2}
do
    ./kube-lock default lock --holder-name holder-${i} --create-configmap --max-holders 3 \
        --polling-interval 2s --polling-timeout 15s \
        --kubeconfig-path ~/.kube/config \
        --unlock
done

kubectl get cm lock -n default -o json | jq -r '.metadata.annotations["kube-lock"]' | jq -re .
