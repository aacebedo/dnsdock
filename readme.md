[![Build Status](https://secure.travis-ci.org/tonistiigi/dnsdock.png)](http://travis-ci.org/tonistiigi/dnsdock)


## dnsdock

DNS server for automatic docker container discovery. Simplified version of [crosbymichael/skydock](https://github.com/crosbymichael/skydock).

#### Differences from skydock

- *No raft / simple in-memory storage* - Does not use any distributed storage and is meant to be used only inside single host. This means no ever-growing log files and memory leakage. AFAIK skydock currently does not have a state machine so the raft log always keeps growing and you have to recreate the server periodically if you wish to run it for a long period of time. Also the startup is very slow because it has to read in all the previous log files.

- *No TTL heartbeat* - Skydock sends heartbeats for every container that reset the DNS TTL value. In production this has not turned out to be reliable. What makes this worse it that if a heartbeat has been missed, skydock does not recover until you restart it. Dnsdock uses static TTL that does not count down. You can override it for a container and also change it without restarting(before updates). In most cases you would want to use TTL=0 anyway.

- *No dependency to other container* - Dnsdock does not use a separate DNS server but has one built in. Linking to another container makes recovery from crash much harder. For example skydock does not recover from skydns crash even if the crashed container is restarted.

- A records only for now.

- No support for Javascript plugins.

- There's a slight difference in a way image names are extracted from a container. Skydock uses the last tag set on image while dnsdock uses the specific tag that was used when the container was created. This means that if a new version of an image comes out and untags the image that your container still uses, the DNS requests for this old container still work.

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

#### Docker Compose Usage

Here's a simple example for the `docker-compose.yml` file

    dnsdock:
        image: tonistiigi/dnsdock
        volumes:
            - /var/run/docker.sock:/run/docker.sock
        ports:
            - 172.17.42.1:53:53/udp
    serviceb:
        image: username/serviceb
        dns: 172.17.42.1

    servicea:
        image: username/servicea
        dns: 172.17.42.1

Services can access each other with: `<container-name>.<image-name>.docker` hostname. You can check `<container-name>` and `<image-name>` after `docker-compose up` in dnsdock logs.

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


#### SELinux and Fedora / RHEL / CentOS

Mounting docker daemon's unix socket may not work with default configuration on these platforms. Please use [selinux-dockersock](https://github.com/dpw/selinux-dockersock) to fix this. More information in [#11](https://github.com/tonistiigi/dnsdock/issues/11).

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


#### Overrides from ENV metadata

If you wish to fine tune the DNS response addresses you can define specific environment variables during container startup. This overrides the default matching scheme from container and image name.

Supported ENV variables are `DNSDOCK_NAME`, `DNSDOCK_IMAGE`, `DNSDOCK_ALIAS`, `DNSDOCK_TTL`.

```
docker run -e DNSDOCK_NAME=master -e DNSDOCK_IMAGE=mysql -e DNSDOCK_TTL=10 \
           --name mymysql mysqlimage
# matches master.mysql.docker
```

```
docker run -e DNSDOCK_ALIAS=db.docker,sql.docker -e DNSDOCK_TTL=10 \
           --name mymysql mysqlimage
# matches db.docker and sql.docker
```

Service metadata syntax by [progrium/registrator](https://github.com/progrium/registrator) is also supported.

```
docker run -e SERVICE_TAGS=master -e SERVICE_NAME=mysql -e SERVICE_REGION=us2 \
           --name mymysql mysqlimage
# matches master.mysql.us2.docker
```

If you want dnsdock to skip processing a specific container set its `DNSDOCK_IGNORE` or `SERVICE_IGNORE` environment variable.


#### OSX Usage

Original tutorial: http://www.asbjornenge.com/wwc/vagrant_skydocking.html

If you use docker on OSX via Vagrant you can do this to make your containers discoverable from your main machine.

In your Vagrantfile add the following to let your virtual machine accept packets for other IPs:

```ruby
config.vm.provider :virtualbox do |vb|
  vb.customize ["modifyvm", :id, "--nicpromisc2", "allow-all"]
end
```

Then route traffic that matches you containers to your virtual machine internal IP:

```
sudo route -n add -net 172.17.0.0 <VAGRANT_MACHINE_IP>
```

Finally, to make OSX use dnsdock for requests that match your domain suffix create a file with your domain ending under `/etc/resolver` (for example `/etc/resolver/myprojectname.docker`) and set its contents to `nameserver 172.17.42.1`.

---

#### Lots of code in this repo is directly influenced by skydns and skydock. Many thanks to the authors of these projects.


