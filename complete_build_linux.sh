#!/usr/bin/env bash

if [ "$#" -ne 1 ]; then
    echo "Usage: $0 <python_version>"
    echo "Example: $0 3.15"
    exit 1
fi

set -euo pipefail

# You run this on a Linux x86_64 machine
# some pre-reqs:
# sudo apt update
# sudo apt install -y build-essential libssl-dev libreadline-dev libxz-dev libbz2-dev pkg-config

# Build the Linux x86_64 binary from the root of the repo (script assumed to live in Linux/ or similar)
cd cpython && (make clean || true) && git clean -fdx -e Doc/venv
mkdir -p build-linux
cd build-linux

export OPENSSL_DIR="/usr"
export READLINE_DIR="/usr"
export XZ_DIR="/usr"
export BZIP2_DIR="/usr"

# Use system libraries for portable build
export CPPFLAGS="-I$OPENSSL_DIR/include -I$READLINE_DIR/include -I$XZ_DIR/include -I$BZIP2_DIR/include"
export LDFLAGS="-L$OPENSSL_DIR/lib/x86_64-linux-gnu -L$READLINE_DIR/lib/x86_64-linux-gnu -L$XZ_DIR/lib/x86_64-linux-gnu -L$BZIP2_DIR/lib/x86_64-linux-gnu"
export PKG_CONFIG_PATH="$OPENSSL_DIR/lib/x86_64-linux-gnu/pkgconfig:$READLINE_DIR/lib/x86_64-linux-gnu/pkgconfig:$XZ_DIR/lib/x86_64-linux-gnu/pkgconfig:$BZIP2_DIR/lib/x86_64-linux-gnu/pkgconfig"

export PREFIX="$PWD/../out/prefix"

../configure \
  --prefix="$PREFIX" \
  --with-openssl="$OPENSSL_DIR" \
  --with-readline=readline \
  --with-ensurepip=install \
  --enable-optimizations

# build and install
make -j"$(nproc)"
make install

# stage for packaging so we can modify without touching build output
cd ..
rm -rf stage
mkdir -p stage
cp -a out/prefix stage/

# -------------------------
# Packaging / size reduction
# -------------------------
cd stage
PREFIX="$(pwd)/prefix"
PYVER="$1"

# Strip main executables + .so files (best size win)
find "$PREFIX" -type f \( -perm -111 -o -name "*.so*" \) -print0 \
  | xargs -0 -n1 strip 2>/dev/null || true

# Remove tests, caches and compiled files
rm -rf "$PREFIX/lib/python$PYVER/test" \
       "$PREFIX/lib/python$PYVER/idlelib/idle_test" \
       "$PREFIX/lib/python$PYVER/unittest/test" || true
find "$PREFIX" -name "__pycache__" -type d -prune -exec rm -rf {} + 2>/dev/null || true
find "$PREFIX" -name "*.pyc" -delete || true
find "$PREFIX" -name "*.pyo" -delete || true

# Remove static libraries
find "$PREFIX" -name "*.a" -delete || true

# remove ootb pip in the bin folder because it's problematic
echo "Removing pip from bin to avoid issues with shebangs and hardcoded paths"
rm -rf prefix/bin/pip*

# (Optional) Remove headers if you never compile extensions on the target
# rm -rf "$PREFIX/include"

# Final tarball (max gzip compression)
GZIP=-9 tar -czf linux-x86_64.tar.gz prefix

echo "Built: $(pwd)/linux-x86_64.tar.gz moving to universal bucket..."
mv linux-x86_64.tar.gz ../../universal-bucket/linux-x86_64.tar.gz
