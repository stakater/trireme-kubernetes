docker run \
  --name "Trireme" \
  --privileged \
  --net host \
  -t \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /root/.kube/config:/root/.kube/config
  --restart always
926088932149.dkr.ecr.us-west-2.amazonaws.com/kubernetes-integration
