# Android SDK 36 and JDK 21 for Capacitor's Gradle build, with Bun for Vite.
FROM oven/bun:1.4.4-debian@sha256:93d5032fe06d163427822dc1598f420363759bb18155c9cc90eb1be05a64a19e AS bun

FROM ghcr.io/cirruslabs/android-sdk:36

COPY --from=bun /usr/local/bin/bun /usr/local/bin/bun
RUN ln -s /usr/local/bin/bun /usr/local/bin/bunx
