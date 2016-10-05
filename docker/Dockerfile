FROM ubuntu
MAINTAINER Bernard Van De Walle <bernard@aporeto.com>

ADD kubernetes-integration /opt/kubernetes-integration/
ADD run.sh /opt/kubernetes-integration/run.sh

RUN mkdir -p /opt/kubernetes-integration
RUN apt-get update && apt-get install -y libnetfilter-queue-dev

WORKDIR /opt/kubernetes-integration
CMD ./run.sh
