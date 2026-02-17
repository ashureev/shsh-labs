FROM ubuntu:22.04

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive

# Install essential tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash=* \
    ca-certificates=* \
    sudo=* \
    curl=* \
    git=* \
    vim=* \
    nano=* \
    jq=* \
    bc=* \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user 'learner' (UID 1000)
RUN groupadd -g 1000 learner && \
    useradd -u 1000 -g 1000 -m -s /bin/bash learner

# Configure sudo for passwordless access
RUN echo "learner ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/learner && \
    chmod 0440 /etc/sudoers.d/learner

# Install bash-preexec for enhanced OSC 133 support
RUN git clone --depth 1 https://github.com/rcaloras/bash-preexec /opt/bash-preexec

# Copy logging scripts with OSC 133 support
COPY --chown=learner:learner container/logging/.bashrc_osc133 /home/learner/.bashrc_logging
COPY --chown=learner:learner container/logging/log_rotate.sh /home/learner/log_rotate.sh

# Make log_rotate.sh executable and configure .bashrc
RUN chmod +x /home/learner/log_rotate.sh && \
    printf '\n# SHSH Session Logging with OSC 133\nif [[ -f /opt/bash-preexec/bash-preexec.sh ]]; then\n    source /opt/bash-preexec/bash-preexec.sh\nfi\nif [[ -f ~/.bashrc_logging ]]; then\n    source ~/.bashrc_logging\nfi\n' >> /home/learner/.bashrc

# Set up working directory
USER learner
WORKDIR /home/learner/work

# Keep container alive
ENTRYPOINT ["/bin/sleep", "infinity"]
