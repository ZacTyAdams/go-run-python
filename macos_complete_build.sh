#!/usr/bin/env bash
set -euo pipefail

# You run this on a local mac machine
# some pre-reqs:
# xcode-select --install
# brew update
# brew install openssl@3 readline xz bzip2 pkg-config
#
# NOTE: We intentionally do NOT use Homebrew zlib to avoid embedding /opt/homebrew paths.
# We rely on macOS system zlib so the build is portable.

# Build the macos binary from the root of the repo (script assumed to live in Android/ or similar)
cd .. && (make clean || true) && git clean -fdx -e Doc/venv
mkdir -p build-macos
cd build-macos

export OPENSSL_DIR="$(brew --prefix openssl@3)"
export READLINE_DIR="$(brew --prefix readline)"
export XZ_DIR="$(brew --prefix xz)"
export BZIP2_DIR="$(brew --prefix bzip2)"

# IMPORTANT: Do NOT include zlib paths here; it will cause extension modules to link to /opt/homebrew/opt/zlib/...
export CPPFLAGS="-I$OPENSSL_DIR/include -I$READLINE_DIR/include -I$XZ_DIR/include -I$BZIP2_DIR/include"
export LDFLAGS="-L$OPENSSL_DIR/lib -L$READLINE_DIR/lib -L$XZ_DIR/lib -L$BZIP2_DIR/lib"
export PKG_CONFIG_PATH="$OPENSSL_DIR/lib/pkgconfig:$READLINE_DIR/lib/pkgconfig:$XZ_DIR/lib/pkgconfig:$BZIP2_DIR/lib/pkgconfig"

export PREFIX="$PWD/../out/prefix"

../configure \
  --prefix="$PREFIX" \
  --with-openssl="$OPENSSL_DIR" \
  --with-readline=readline \
  --with-ensurepip=install \
  --enable-optimizations

# build and install
make -j"$(sysctl -n hw.ncpu)"
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
PYVER="3.15"

# Strip main executables + dylibs + extension modules (best size win)
find "$PREFIX" -type f \( -perm -111 -o -name "*.dylib" -o -name "*.so" \) -print0 \
  | xargs -0 -n1 strip -x 2>/dev/null || true

# Remove tests, caches and compiled files
rm -rf "$PREFIX/lib/python$PYVER/test" \
       "$PREFIX/lib/python$PYVER/idlelib/idle_test" \
       "$PREFIX/lib/python$PYVER/unittest/test" || true
find "$PREFIX" -name "__pycache__" -type d -prune -exec rm -rf {} + 2>/dev/null || true
find "$PREFIX" -name "*.pyc" -delete || true
find "$PREFIX" -name "*.pyo" -delete || true

# Remove static libraries
find "$PREFIX" -name "*.a" -delete || true

# (Optional) Remove headers if you never compile extensions on the target
# rm -rf "$PREFIX/include"

# Final tarball (max gzip compression)
GZIP=-9 tar -czf darwin-arm64.tar.gz prefix

echo "Built: $(pwd)/darwin-arm64.tar.gz"
