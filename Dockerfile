# Testing the arm 64 build process
FROM --platform=linux/amd64 python:3.14.3-bookworm
# FROM --platform=linux/arm64 python:3.14.3-bookworm

ENV DEBIAN_FRONTEND=noninteractive
ENV ANDROID_SDK_ROOT=/opt/android-sdk
ENV ANDROID_HOME=/opt/android-sdk
ENV PATH=$PATH:/opt/android-sdk/cmdline-tools/latest/bin:/opt/android-sdk/platform-tools
ENV HOST=aarch64-linux-android

# Install system packages needed to build CPython and run Android tools
 RUN apt-get update \
     && apt-get install -y --no-install-recommends \
         ca-certificates curl wget unzip git build-essential clang pkg-config \
         libssl-dev libbz2-dev libreadline-dev libsqlite3-dev libncurses-dev \
         xz-utils liblzma-dev zlib1g-dev libffi-dev cmake ninja-build \
         openjdk-17-jdk-headless sudo acl patchelf \
     && rm -rf /var/lib/apt/lists/*

# Create SDK directories
RUN mkdir -p ${ANDROID_SDK_ROOT}/cmdline-tools && mkdir -p ${ANDROID_SDK_ROOT}/licenses
WORKDIR /tmp

# Download Android command line tools (Linux). If Google updates the filename you
# may need to change the URL to the latest commandlinetools ZIP for Linux.
ARG CMDLINE_TOOLS_URL=https://dl.google.com/android/repository/commandlinetools-linux-9477386_latest.zip
RUN curl -fSL "$CMDLINE_TOOLS_URL" -o /tmp/cmdline-tools.zip \
    && unzip /tmp/cmdline-tools.zip -d /tmp/cmdline-tools-tmp \
    && mkdir -p ${ANDROID_SDK_ROOT}/cmdline-tools/latest \
    && mv /tmp/cmdline-tools-tmp/cmdline-tools/* ${ANDROID_SDK_ROOT}/cmdline-tools/latest/ \
    && rm -rf /tmp/cmdline-tools.zip /tmp/cmdline-tools-tmp

# Ensure sdkmanager is executable and usable
RUN chmod +x ${ANDROID_SDK_ROOT}/cmdline-tools/latest/bin/*



# Install Android packages (NDK version is matched to android-env.sh).
# This step requires Java (installed above). Adjust NDK version if required.
ARG ANDROID_NDK_VERSION=27.3.13750724
RUN yes | ${ANDROID_SDK_ROOT}/cmdline-tools/latest/bin/sdkmanager --sdk_root=${ANDROID_SDK_ROOT} "platform-tools" "platforms;android-24" "ndk;${ANDROID_NDK_VERSION}"

# Install golang
COPY go1.25.6.linux-amd64.tar.gz /go1.25.6.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf /go1.25.6.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"


# Create a non-root developer user and set workspace
ARG UNAME=dev
ARG UID=1000
RUN useradd -m -u ${UID} -s /bin/bash ${UNAME} && echo "${UNAME} ALL=(ALL) NOPASSWD:ALL" >/etc/sudoers.d/${UNAME}
USER ${UNAME}
WORKDIR /local-volume-bridge

# Python tooling for host-side helper scripts
RUN python3 -m pip install --user --upgrade pip setuptools wheel

# Default environment useful for builds
ENV JAVA_HOME=/usr/lib/jvm/java-17-openjdk-amd64
# ENV PATH=$HOME/.local/bin:$PATH

# Drop-in entrypoint for convenience (override as needed)
ENTRYPOINT ["/bin/bash"]
