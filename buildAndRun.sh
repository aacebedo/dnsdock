docker stop dnsdock
docker rm dnsdock
docker build -t tonistiigi/dnsdock .
docker run -d -p "80:80" -p "53:53/udp" --name dnsdock -v /var/run/docker.sock:/var/run/docker.sock tonistiigi/dnsdock
