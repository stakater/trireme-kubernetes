FROM ubuntu
MAINTAINER Bernard Van De Walle <bernard@aporeto.com>

RUN mkdir -p /opt/kubernetes-integration
RUN apt-get update && apt-get install -y libnetfilter-queue-dev

ADD kubernetes-integration /opt/kubernetes-integration/
ADD run.sh /opt/kubernetes-integration/run.sh

WORKDIR /opt/kubernetes-integration
CMD ./run.sh
