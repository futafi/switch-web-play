FROM debian:12

ENV DEBIAN_FRONTEND=noninteractive \
    LIBVA_DRIVER_NAME=iHD \
    GST_VAAPI_ALL_DRIVERS=1

RUN sed -i 's/Components: main/Components: main contrib non-free non-free-firmware/' /etc/apt/sources.list.d/debian.sources \
 && apt-get update \
 && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    build-essential \
    meson \
    ninja-build \
    pkg-config \
    libglib2.0-dev \
    libgstreamer1.0-dev \
    libgstreamer-plugins-base1.0-dev \
    libjson-glib-dev \
    libdaemon-dev \
    libjansson-dev \
    libedit-dev \
    libsoup2.4-dev \
    gstreamer1.0-tools \
    gstreamer1.0-alsa \
    gstreamer1.0-plugins-base \
    gstreamer1.0-plugins-good \
    gstreamer1.0-plugins-bad \
    gstreamer1.0-plugins-ugly \
    gstreamer1.0-libav \
    gstreamer1.0-rtsp \
    gstreamer1.0-vaapi \
    libgstreamer-plugins-bad1.0-0 \
    libva2 \
    libva-drm2 \
    libva-x11-2 \
    libva-wayland2 \
    libdrm2 \
    libdrm-intel1 \
    libmfx1 \
    intel-media-va-driver-non-free \
    vainfo \
 && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 https://github.com/RidgeRun/gstd-1.x.git /opt/gstd-1.x \
 && cd /opt/gstd-1.x \
 && meson setup build --prefix=/usr -Denable-tests=disabled -Denable-examples=disabled -Denable-systemd=disabled \
 && ninja -C build \
 && ninja -C build install

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
