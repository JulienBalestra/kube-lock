# kube-lock [![CircleCI](https://circleci.com/gh/JulienBalestra/kube-lock.svg?style=svg)](https://circleci.com/gh/JulienBalestra/kube-lock) [![Docker Repository on Quay](https://quay.io/repository/julienbalestra/kube-lock/status "Docker Repository on Quay")](https://quay.io/repository/julienbalestra/kube-lock)

kube-lock allows to proceed to a semaphore over a configmap.

Have a look to the [docs](docs).

```bash
NAMESPACE="default"
CONFIGMAP_NAME="lock"

./kube-lock ${NAMESPACE} ${CONFIGMAP_NAME} \
    --holder-name holder-1 --create-configmap --max-holders 1 \
    --polling-interval 10s --polling-timeout 5m \
    --reason "db restart" \
    --kubeconfig-path ${HOME}/.kube/config
```

```text
I0819 19:09:54.017121   31587 kubelock.go:123] Processing lock over cm/lock in ns default for reason: "db restart"
I0819 19:09:54.029022   31587 kubelock.go:134] Creating cm/lock in ns default
I0819 19:09:54.032793   31587 kubelock.go:163] Can lock semaphore in cm/lock in ns default: 0/1 holders
I0819 19:09:54.034440   31587 kubelock.go:169] Successfully locked cm/lock in ns default for holder holder-1
```

Observe the lock:
```bash
kubectl get cm lock -n default -o json | jq -r '.metadata.annotations["kube-lock"]' | jq .
```
```json
{
  "max": 1,
  "holders": {
    "holder-1": {
      "date": "2018-08-19T19:14:24Z",
      "reason": "db restart"
    }
  }
}
```

Then lock with `holder-2`:
```bash
./kube-lock ${NAMESPACE} ${CONFIGMAP_NAME} \
    --holder-name holder-2 --create-configmap --max-holders 2 \
    --polling-interval 10s --polling-timeout 25s \
    --reason "db restart" \
    --kubeconfig-path ${HOME}/.kube/config
```

```text
I0819 19:12:02.974888   31946 kubelock.go:123] Processing lock over cm/lock in ns default for reason: "db restart"
I0819 19:12:02.991963   31946 kubelock.go:160] Cannot lock semaphore in cm/lock in ns default: 1/1 holders
I0819 19:12:02.991981   31946 kubelock.go:183] Starting to poll for lock every 10s, timeout after 25s
I0819 19:12:12.992122   31946 kubelock.go:123] Processing lock over cm/lock in ns default for reason: "db restart"
I0819 19:12:12.994847   31946 kubelock.go:160] Cannot lock semaphore in cm/lock in ns default: 1/1 holders
I0819 19:12:12.994878   31946 kubelock.go:215] Semaphore is full, skipping lock
I0819 19:12:22.992131   31946 kubelock.go:123] Processing lock over cm/lock in ns default for reason: "db restart"
I0819 19:12:22.994867   31946 kubelock.go:160] Cannot lock semaphore in cm/lock in ns default: 1/1 holders
I0819 19:12:22.994895   31946 kubelock.go:215] Semaphore is full, skipping lock
E0819 19:12:27.992329   31946 cmd.go:67] Command returns error: cannot lock cm/lock in ns default, timeout after 25s
E0819 19:12:27.993041   31946 main.go:23] Exiting on error: 2
```

To let `holder-2` takes its lock, `holder-1` need to release - unlock its previous hold:
```bash
./kube-lock ${NAMESPACE} ${CONFIGMAP_NAME} \
    --holder-name holder-1 --unlock \
    --kubeconfig-path ${HOME}/.kube/config
```

```text
I0819 19:13:54.808035   32211 kubelock.go:225] Processing unlock over cm/lock in ns default
I0819 19:13:54.823725   32211 kubelock.go:243] Unlock current holder holder-1 took on 2018-08-19T19:09:54Z with reason: "db restart"
I0819 19:13:54.826082   32211 kubelock.go:249] Successfully unlocked cm/lock in ns default for holder holder-1
```

We can observe the absence of any holders:
```bash
kubectl get cm lock -n default -o json | jq -r '.metadata.annotations["kube-lock"]' | jq -re .
```

```json
{
  "max": 1,
  "holders": {}
}
```
