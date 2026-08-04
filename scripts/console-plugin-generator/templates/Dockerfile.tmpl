FROM registry.access.redhat.com/ubi9/nodejs-20:latest AS build
USER root
# Install yarn if not available, with security updates
RUN dnf update -y && dnf clean all && (command -v yarn || npm i -g yarn)

# Copy only package files first for better caching
COPY package.json yarn.lock* /usr/src/app/
WORKDIR /usr/src/app
RUN yarn install --frozen-lockfile --production=false

# Copy source and build
COPY . /usr/src/app
RUN yarn build

FROM registry.access.redhat.com/ubi9/nginx-120:latest

# Apply security updates
USER root
RUN dnf update -y && dnf clean all

# Copy built assets with proper ownership
COPY --from=build --chown=1001:0 /usr/src/app/dist /usr/share/nginx/html

# Switch to non-root user
USER 1001

# Use exec form for proper signal handling
ENTRYPOINT ["nginx", "-g", "daemon off;"]

# Add security labels and metadata
LABEL security.non-root=true \
      security.disable-users=root \
      security.openshift.io/required-scc=restricted-v2
