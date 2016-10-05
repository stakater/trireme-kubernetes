FROM ubuntu
MAINTAINER Bernard Van De Walle <bernard@aporeto.com>

RUN mkdir -p /opt/kubernetes-integration
RUN apt-get update && apt-get install -y \
    libnetfilter-queue-dev \
    iptables \
&& rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ADD app /opt/kubernetes-integration/
ADD run.sh /opt/kubernetes-integration/run.sh

WORKDIR /opt/kubernetes-integration
CMD ./run.sh
