FROM ubuntu
MAINTAINER Bernard Van De Walle <bernard@aporeto.com>

RUN mkdir -p /opt/trireme-kubernetes
RUN apt-get update && apt-get install -y \
    libnetfilter-queue-dev \
    iptables \
    ipset \
&& rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ADD trireme-kubernetes /opt/trireme/trireme-kubernetes
ADD run.sh /opt/trireme/run.sh

WORKDIR /opt/trireme
CMD ["/trireme-kubernetes"] 
