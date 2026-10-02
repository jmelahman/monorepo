# Android SDK 36 and JDK 21 for Capacitor's Gradle build, with Bun for Vite.
FROM oven/bun:1.4.2-debian@sha256:4f6e31d1a54d6a3dd312daef655fc998101b5043d52e12592ac293ef04b9bc73 AS bun

FROM ghcr.io/cirruslabs/android-sdk:36

COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s /usr/local/bin/bun /usr/local/bin/bunx
