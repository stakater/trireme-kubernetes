FROM ubuntu
MAINTAINER Bernard Van De Walle <bernard@aporeto.com>

RUN mkdir -p /opt/trireme-kubernetes
RUN apt-get update && apt-get install -y \
    libnetfilter-queue-dev \
    iptables \
&& rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ADD app /opt/trireme-kubernetes/
ADD run.sh /opt/trireme-kubernetes/run.sh

WORKDIR /opt/trireme-kubernetes
CMD ./run.sh
