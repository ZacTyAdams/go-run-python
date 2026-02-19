#!/usr/bin/env bash

set -euo pipefail

PYTHON_VERSION="${1:-3.14}"
WORK_DIR="/tmp/python-build-$$"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building Python $PYTHON_VERSION for Linux x86_64..."
echo "Work directory: $WORK_DIR"

# Ensure prerequisites are installed (skip by default inside container)
echo "Checking build dependencies..."
if [ "${INSTALL_DEPS:-0}" = "1" ] && command -v apt-get &> /dev/null; then
  APT_GET="apt-get"
  APT_PREFIX=""
  if [ "$(id -u)" -eq 0 ]; then
    APT_PREFIX=""
  elif sudo -n true 2>/dev/null; then
    APT_PREFIX="sudo"
  else
    echo "Skipping apt-get (no root/sudo access). Ensure these are installed:"
    echo "  build-essential libssl-dev libreadline-dev libxz-dev (or liblzma-dev) libbz2-dev"
    echo "  libffi-dev libncursesw5-dev libsqlite3-dev zlib1g-dev wget ca-certificates"
    APT_GET=""
  fi

  if [ -n "$APT_GET" ]; then
    echo "Installing build dependencies..."
    $APT_PREFIX $APT_GET update -qq
    $APT_PREFIX $APT_GET install -y -qq \
      build-essential \
      libssl-dev \
      libreadline-dev \
      libbz2-dev \
      libffi-dev \
      libncursesw5-dev \
      libsqlite3-dev \
      zlib1g-dev \
      wget \
      ca-certificates

    # Try libxz-dev, fall back to liblzma-dev if not available
    if ! $APT_PREFIX $APT_GET install -y -qq libxz-dev; then
      $APT_PREFIX $APT_GET install -y -qq liblzma-dev
    fi
  fi
else
  echo "Assuming build dependencies are pre-installed in container."
  echo "Required: build-essential libssl-dev libreadline-dev libxz-dev libbz2-dev"
  echo "          libffi-dev libncursesw5-dev libsqlite3-dev zlib1g-dev wget ca-certificates"
fi

# remove existing pip
rm -rf /home/dev/.local/lib/python3.14
# Create work directory
mkdir -p "$WORK_DIR"
cd "$WORK_DIR"

# Use CPython submodule (no download)
PYTHON_MAJOR_MINOR="${PYTHON_VERSION%.*}"
CPYTHON_SRC="$REPO_ROOT/cpython"
if [ ! -d "$CPYTHON_SRC" ]; then
  echo "Error: cpython submodule not found at $CPYTHON_SRC"
  exit 1
fi
echo "Using CPython source at $CPYTHON_SRC"
BUILD_DIR="$WORK_DIR/build-linux"
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

# Configure with system libraries for portability
echo "Configuring CPython..."
"$CPYTHON_SRC/configure" \
  --prefix="$WORK_DIR/install" \
  --with-openssl=/usr \
  --disable-test-modules \
  --with-openssl-rpath=no \
  --with-readline \
  --with-ensurepip=install \
  --enable-optimizations \
  --enable-shared

# Build and install
echo "Building CPython (this may take a few minutes)..."
make -j"$(nproc)"
make install

# Stage for packaging
STAGE_DIR="$WORK_DIR/stage"
mkdir -p "$STAGE_DIR"
cp -a "$WORK_DIR/install" "$STAGE_DIR/python"

cd "$STAGE_DIR"

# Strip binaries and shared objects for size
echo "Stripping symbols for size reduction..."
# find python -type f \( -perm -111 -o -name "*.so*" \) -exec strip {} + 2>/dev/null || true

# Remove tests and caches
echo "Removing test suites and caches..."
rm -rf "python/lib/python$PYTHON_MAJOR_MINOR/test" \
       "python/lib/python$PYTHON_MAJOR_MINOR/idlelib/idle_test" \
       "python/lib/python$PYTHON_MAJOR_MINOR/unittest/test"
# find python -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
# find python -type f -name "*.pyc" -delete 2>/dev/null || true
# find python -type f -name "*.pyo" -delete 2>/dev/null || true

# Remove static libraries
# find python -type f -name "*.a" -delete 2>/dev/null || true

# Remove pip executables (problematic shebangs)
# rm -f python/bin/pip*

# copy in the dependencies
echo "Copying found depenencies"
# cp /lib64/libpython3.14.so.1.0 python/lib/
cp /lib64/libm.so.6 python/lib/
cp /lib64/libc.so.6 python/lib/
cp /lib64/ld-linux-x86-64.so.2 python/lib/
cp /lib64/liblzma* python/lib/
cp /lib64/libssl.so.3 python/lib/
cp /lib64/libcrypto.so.3 python/lib/
cp /lib64/libz.so.1 python/lib/
cp "$REPO_ROOT/python-launcher/python-launcher-linux-amd64" python/bin/python-launcher
chmod +x python/bin/python-launcher
cp "$REPO_ROOT/patchelf" python/bin/patchelf
chmod +x python/bin/patchelf

# Create tarball
echo "Creating tarball..."
mkdir -p "$REPO_ROOT/universal-bucket"
GZIP=-9 tar -czf "$REPO_ROOT/universal-bucket/linux-x86_64.tar.gz" python

echo "✓ Build complete!"
echo "✓ Output: $REPO_ROOT/universal-bucket/linux-x86_64.tar.gz"
echo "✓ Size: $(du -h "$REPO_ROOT/universal-bucket/linux-x86_64.tar.gz" | cut -f1)"

echo "Performing test of the self contained dependencies"

# Cleanup
cd /
rm -rf "$WORK_DIR"

echo "Done."
