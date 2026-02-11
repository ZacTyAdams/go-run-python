docker rm -f cpython-builder-local 
echo y | docker image prune -a 
PLATFORM="${PLATFORM:-linux/amd64}"
docker build --platform "$PLATFORM" -t cpython-builder .
directory_path=$(realpath .)
docker run --platform "$PLATFORM" -v "$directory_path:/local-volume-bridge" -e HOST_DIR="$directory_path" -d -t --name cpython-builder-local cpython-builder:latest
docker exec -it cpython-builder-local /bin/bash
# docker exec -it iidops-local python /opt/ui/tui.py
