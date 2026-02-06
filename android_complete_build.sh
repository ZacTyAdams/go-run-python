cd cpython/Android
# To cleanup the old build stuff
cd .. && make clean || true && git clean -fdx -e Doc/venv
# standard buld process
./android.py configure-build
./android.py make-build
./android.py configure-host aarch64-linux-android
./android.py make-host aarch64-linux-android

# cd /workspaces/cpython-android && rm -rf stage && mkdir -p stage && cp -a cross-build/aarch64-linux-android/prefix stage/

PYROOT="$(pwd)/stage/prefix"
PYBIN="$PYROOT/bin/python3.15"

cd "$PYROOT/bin" || exit 1

for f in *; do
  [ -f "$f" ] || continue
  head -n 1 "$f" | grep -q '^#!' || continue
  head -n 1 "$f" | grep -qi python || continue
  sed -i "1 s|^#!.*python.*$|#!$PYBIN|" "$f"
done

rm -rf "$PYROOT/share/man"
rm -rf "$PYROOT/lib/python3.15/test"
rm -rf "$PYROOT/lib/python3.15/__pycache__"
find "$PYROOT" -name '*.pyc' -delete

# build the tarball
cd stage
tar -czf python-3.15-android-arm64-prefix.tar.gz prefix
