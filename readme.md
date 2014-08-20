### dnsdock

DNS server for automatic docker container discovery. Simplified version of [crosbymichael/skydock]().

#### Differences from skydock

- *No raft / simple in-memory storage* - Does not use any distributed storage and is meant to be used only inside single host. This means no evergrowing log files and memory leakage. AFAIK skydock currently does not have a state machine so the raft log always keeps growing and you have to recreate the server periodically if you wish to run it for a long period of time. Also the startup is very slow because it has to read in all the previous log files.

- *No TTL hearthbeat* - Skydock sends hearthbeats for every container that reset the DNS TTL value. In production this has not turned out to be reliable. What makes this worse it that if a hearthbeat has been missed skydock does not recover until you restart it. Dnsdock uses static TTL that does not count down. You can override it for a container and also change it without restarting(before updates). In most cases you would want to use TTL=0 anyway.

- *No dependency to other container* - Dnsdock does not use a separate DNS server but has one built in. Linking to another container makes recovery from crash much harder. For example skydock does not recover from skydns crash even if the crashed container is restarted.