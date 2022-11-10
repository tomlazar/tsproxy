# tsproxy

Forwards a https connection from PORT=XYZ HOSTNAME=XYZ -> REMOTE=(XYZ or just the same as port if you leave blank).

Add a DEBUG=full if you like logs.

## Paramters

| ENV_VAR  | Description                                                                                                                                                                                                        |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| LISTEN   | Configure where to listen for conects, in the form of addr=:80,port=80,tls=off,network=tcp; where a semi colon can be used to split multiple sockets to listen on. All the values exepct a port/addr are optional. |
| REMOTE   | The remote host to proxy too                                                                                                                                                                                       |
| HOSTNAME | The tailscale hostname to use                                                                                                                                                                                      |
| STRATEGY | Controls what flags are set upstream when connecting to the `REMOTE`                                                                                                                                               |
| DEBUG    | Print out all the things I once described useful                                                                                                                                                                   |


## Goals

+ don't have more then 1 file (tests don't count (but config does?))
+ I guess support different well know proxy confgiuration `STRATEGY`'s like (docker)