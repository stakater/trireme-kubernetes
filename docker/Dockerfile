FROM alpine
MAINTAINER Antoine Mercadal <antoine@aporeto.com>

ADD kubernetes-integration /opt/kubernetes-integration/
ADD run.sh /opt/kubernetes-integration/run.sh

RUN mkdir -p /opt/kubernetes-integration

WORKDIR /opt/kubernetes-integration
CMD ./run.sh
