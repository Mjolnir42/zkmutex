# WORK IN PROGRESS

Do not touch. Slippery. Wet. Harmful. Not for oral consumption.

# zkmutex

Zookeeper Mutex - run a command under a distributed mutex

`zkmutex` runs `${cmd}` after it has successfully acquired the
required mutex. The mutex path is calculated over the Blake2b-512
hash of the command that should be run.

This means that the following commands on different hosts will run
concurrently:

```
zkmutex -- puppet apply
zkmutex -- shutdown -r now
```

Multiple hosts trying to run either of them will get serialized.

# Zookeeper layout

```
/
└── zkmutex
    └── <syncgroup>
        └── <hash>
            ├── last
            ├── command
            └── runlock/
```

# Configuration

```
/etc/zkmutex/zkmutex.conf:
ensemble: <zk-connect-string>
syncgroup: <name>
```

# Execute

```
zkmutex -- ${cmd}
zkmutex --config /tmp/foo.conf -- ${cmd}
```
