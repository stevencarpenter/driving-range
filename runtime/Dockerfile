# check=skip=SecretsUsedInArgOrEnv

# GIT_CONFIG_KEY_* names public Git settings, not secrets. Keep their runtime overrides.
FROM debian:trixie-slim@sha256:d7e12182ce18b85b93007c1dedf31f2d29e01ccf3182cc4017c709b6259bc132
ARG TARGETARCH
# Security fixes for packages inherited from the pinned base (September 2026 audit).
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates curl neovim ripgrep fd-find findutils grep sed mawk git python3 zsh fzf tmux \
    gzip=1.13-1+deb13u1 libc6=2.41-12+deb13u4 libc-bin=2.41-12+deb13u4 \
    libpcre2-8-0=10.46-1~deb13u3 libsqlite3-0=3.46.1-7+deb13u2 \
    && rm -rf /var/lib/apt/lists/* \
    && ln -s /usr/bin/fdfind /usr/local/bin/fd
RUN case "$TARGETARCH" in \
      arm64) arch=aarch64; hash=60d42fa2a9abaa445eff10cd2087458562aaad5a54b90309e5a3787ecc985ff2 ;; \
      amd64) arch=x86_64; hash=0a07bab4641a55fd2bc2fd1563ba3a3f9a577584086ad74086a1c5b69b3ffce9 ;; \
      *) exit 1 ;; \
    esac \
    && curl -fsSL "https://github.com/jj-vcs/jj/releases/download/v0.44.0/jj-v0.44.0-${arch}-unknown-linux-musl.tar.gz" -o /tmp/jj.tar.gz \
    && echo "$hash  /tmp/jj.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/jj.tar.gz -C /usr/local/bin ./jj \
    && rm /tmp/jj.tar.gz
RUN useradd --uid 1000 --create-home golf && mkdir /workspace && chown golf:golf /workspace \
    && dpkg-query -W > /usr/local/share/golf-packages.txt
COPY golf-shell golf-brief golf-check golf-hint /usr/local/bin/
RUN chmod 755 /usr/local/bin/golf-shell /usr/local/bin/golf-brief /usr/local/bin/golf-check /usr/local/bin/golf-hint
ENV HOME=/tmp/golf-home LANG=C.UTF-8 LC_ALL=C.UTF-8 TERM=xterm-256color \
    GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null \
    GIT_AUTHOR_NAME="Golf Player" GIT_AUTHOR_EMAIL=player@example.invalid \
    GIT_COMMITTER_NAME="Golf Player" GIT_COMMITTER_EMAIL=player@example.invalid \
    GIT_CONFIG_COUNT=4 GIT_CONFIG_KEY_0=core.hooksPath GIT_CONFIG_VALUE_0=/dev/null \
    GIT_CONFIG_KEY_1=credential.helper GIT_CONFIG_VALUE_1= \
    GIT_CONFIG_KEY_2=commit.gpgsign GIT_CONFIG_VALUE_2=false \
    GIT_CONFIG_KEY_3=tag.gpgsign GIT_CONFIG_VALUE_3=false \
    JJ_USER=Golf\ Player JJ_EMAIL=player@example.invalid JJ_CONFIG=/dev/null \
    GIT_TERMINAL_PROMPT=0 GIT_PAGER=cat PAGER=cat
USER 1000:1000
WORKDIR /workspace
CMD ["/bin/sleep", "3600"]
