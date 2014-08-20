[![Build Status](https://secure.travis-ci.org/tonistiigi/dnsdock.png)](http://travis-ci.org/tonistiigi/dnsdock)


## dnsdock

DNS server for automatic docker container discovery. Simplified version of [crosbymichael/skydock](https://github.com/crosbymichael/skydock).

#### Differences from skydock

- *No raft / simple in-memory storage* - Does not use any distributed storage and is meant to be used only inside single host. This means no ever-growing log files and memory leakage. AFAIK skydock currently does not have a state machine so the raft log always keeps growing and you have to recreate the server periodically if you wish to run it for a long period of time. Also the startup is very slow because it has to read in all the previous log files.

- *No TTL heartbeat* - Skydock sends heartbeats for every container that reset the DNS TTL value. In production this has not turned out to be reliable. What makes this worse it that if a heartbeat has been missed, skydock does not recover until you restart it. Dnsdock uses static TTL that does not count down. You can override it for a container and also change it without restarting(before updates). In most cases you would want to use TTL=0 anyway.

- *No dependency to other container* - Dnsdock does not use a separate DNS server but has one built in. Linking to another container makes recovery from crash much harder. For example skydock does not recover from skydns crash even if the crashed container is restarted.

- A records only for now.

- No support for Javascript plugins.

#### Usage

Dnsdock connects to Docker Remote API and keeps an up to date list of running containers. If a DNS request matches some of the containers their local IP addresses are returned.

Format for a request matching a container is:
`<anything>.<container-name>.<image-name>.<environment>.<domain>`.

- `environment` and `domain` are static suffixes that are set on startup. Defaults to `docker`.
- `image-name` is last part of the image tag used when starting the container.
- `container-name` alphanumerical part of container name.

You can always leave out parts from the left side. If multiple containers match then they are all returned. Wildcard requests are also supported.


```
> dig *.docker
...
;; ANSWER SECTION:
docker.			0	IN	A	172.17.42.5
docker.			0	IN	A	172.17.42.3
docker.			0	IN	A	172.17.42.2
docker.			0	IN	A	172.17.42.7

> dig redis.docker
...
;; ANSWER SECTION:
redis.docker.		0	IN	A	172.17.42.3
redis.docker.		0	IN	A	172.17.42.2

> dig redis1.redis.docker
...
;; ANSWER SECTION:
redis1.redis.docker.		0	IN	A	172.17.42.2

> dig redis1.*.docker
...
;; ANSWER SECTION:
redis1.*.docker.		0	IN	A	172.17.42.2
```

#### Setup

DNS listening port needs to be binded to the *docker0* inferface so that its available to all containers. To avoid this IP changing during host restart add it the docker default options. Open file `/etc/default/docker` and add `-bip=172.17.42.1/24 -dns 172.17.42.1` to `DOCKER_OPTS` variable. Restart docker daemon after you have done that (`sudo service docker restart`).

Now you only need to run the dnsdock container:

```
docker run -d -v /var/run/docker.sock:/var/run/docker.sock --name dnsdock -p 172.17.42.1:53:53/udp tonistiigi/dnsdock [--opts]
```

- `-d` starts container as daemon
- `-v /var/run/docker.sock:/var/run/docker.sock` shares the docker socket to the container so that dnsdock can connect to the Docker API.
- `-p 172.17.42.1:53:53/udp` exposes the default DNS port to the docker0 bridge interface.

Additional configuration options to dnsdock command:

```
-dns=":53": Listen DNS requests on this address
-docker="unix://var/run/docker.sock": Path to the docker socket
-domain="docker": Domain that is appended to all requests
-environment="": Optional context before domain suffix
-help=false: Show this message
-http=":80": Listen HTTP requests on this address
-nameserver="8.8.8.8:53": DNS server for unmatched requests
-ttl=0: TTL for matched requests
-verbose=true: Verbose output
```

If you also want to let the host machine discover the containers add `nameserver 172.17.42.1` to your `/etc/resolv.conf`.


#### HTTP Server

For easy overview and manual control dnsdock also includes HTTP server that lets you configure the server using a JSON API.

```
# show all active services
curl http://dnsdock.docker/services

# show a service
curl http://dnsdock.docker/services/serviceid

# add new service manually
curl http://dnsdock.docker/services/newid -X PUT --data-ascii '{"name": "foo", "image": "bar", "ip": "192.168.0.3", "ttl": 30}'

# remove a service
curl http://dnsdock.docker/services/serviceid -X DELETE

# change a property of an existing service
curl http://dnsdock.docker/services/serviceid -X PATCH --data-ascii '{"ttl": 0}'

# set new default TTL value
curl http://dnsdock.docker/set/ttl -X PUT --data-ascii '10'
```

---

#### Lots of code in this repo is directly influenced by skydns and skydock. Many thanks to the authors of these projects.


