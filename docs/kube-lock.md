## kube-lock

Use this command to take a lock over a configmap

### Synopsis

Use this command to take a lock over a configmap

```
kube-lock command line [flags]
```

### Examples

```

kube-lock [namespace] [configmap name]

```

### Options

```
      --create-configmap            create the configmap if not found
  -h, --help                        help for kube-lock
      --holder-name string          holder name, leave empty to use the hostname
      --kubeconfig-path string      kubernetes config path, leave empty for inCluster config
      --max-holders string          max number of holders, must be > 0 (default "1")
      --polling-interval duration   interval between each lock attempt (default 30s)
      --polling-timeout duration    timeout threshold for polling (default 5m0s)
      --reason string               holder name, leave empty to use the hostname
      --run-once                    try to lock once, exit on error if failed
      --unlock                      unlock the semaphore
  -v, --verbose int                 verbose level
```

