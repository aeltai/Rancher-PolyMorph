#!/usr/bin/env bash
# Reconnect RKE1 downstream agents after Rancher restore / hostname change (migration cutover).
#
# After restoring Rancher on a new server, existing RKE1 clusters still point their
# cattle-*-agent workloads at the OLD Rancher URL. This script repoints them to the
# NEW Rancher without rebuilding the cluster.
#
# Run once per RKE1 node in the cluster (each node has its own cattle-node-agent).
#
# Usage: reconnect-rke1-agents.sh <any-node-ip> <rancher-private-ip> <rancher-hostname> [ca-checksum]
#
# Arguments:
#   node-ip            SSH target — any node in the downstream RKE1 cluster
#   rancher-private-ip New Rancher server's VPC private IP (not public)
#   rancher-hostname   New Rancher URL hostname (e.g. rancher.example.com)
#   ca-checksum        Optional SHA-256 of Rancher cacerts; auto-fetched if omitted
#
# Environment:
#   SSH_KEY  Path to SSH private key (default: ~/.ssh/id_ed25519)
set -euo pipefail

NODE_IP="${1:?usage: $0 <node-ip> <rancher-private-ip> <rancher-hostname> [ca-checksum]}"
RANCHER_PRIVATE="${2:?}"
RANCHER_HOST="${3:?}"
# Agents verify the Rancher TLS cert against this checksum (from /v3/settings/cacerts).
CATTLE_CA_CHECKSUM="${4:-$(bash "$(dirname "$0")/rancher-ca-checksum.sh" "https://${RANCHER_HOST}")}"
RANCHER_URL="https://${RANCHER_HOST}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"
# Must match the downstream cluster's Kubernetes version (RKE1 ships no host kubectl).
HYPERKUBE="registry.rancher.com/rancher/hyperkube:v1.32.6-rancher1"

echo "=== Reconnect RKE1 agents via ${NODE_IP} -> ${RANCHER_URL} (${RANCHER_PRIVATE}) ==="

# All cluster changes run ON the RKE1 node via SSH (or replicate these kubectl patches elsewhere).
ssh -i "${SSH_KEY}" -o StrictHostKeyChecking=no "ubuntu@${NODE_IP}" bash -s <<REMOTE
set -euo pipefail
RANCHER_PRIVATE="${RANCHER_PRIVATE}"
RANCHER_HOST="${RANCHER_HOST}"
RANCHER_URL="${RANCHER_URL}"
CATTLE_CA_CHECKSUM="${CATTLE_CA_CHECKSUM}"
HYPERKUBE="${HYPERKUBE}"

# --- 1. Host-level DNS override (VPC hairpin avoidance) ---
# Nodes in the same VPC as Rancher often cannot reach Rancher's public IP from inside.
# Map the new hostname to the private IP on the host (for host-network processes).
grep -q "\${RANCHER_HOST}" /etc/hosts && \\
  sudo sed -i "s/.*\${RANCHER_HOST}.*/\${RANCHER_PRIVATE}  \${RANCHER_HOST}/" /etc/hosts || \\
  echo "\${RANCHER_PRIVATE}  \${RANCHER_HOST}" | sudo tee -a /etc/hosts

# --- 2. Temporary kubectl access to the local RKE1 API ---
# RKE1 nodes don't install kubectl; use hyperkube in Docker with host networking.
SSL=/etc/kubernetes/ssl
TMP=/tmp/rke-admin-kubeconfig
sudo mkdir -p "\$TMP"
if [ ! -f "\$TMP/admin.key" ]; then
  # Short-lived admin client cert signed by the cluster kube-ca (reused on re-runs).
  sudo openssl genrsa -out "\$TMP/admin.key" 2048
  sudo openssl req -new -key "\$TMP/admin.key" -out "\$TMP/admin.csr" -subj "/O=system:masters/CN=restore-admin"
  sudo openssl x509 -req -in "\$TMP/admin.csr" -CA "\$SSL/kube-ca.pem" -CAkey "\$SSL/kube-ca-key.pem" -CAcreateserial -out "\$TMP/admin.crt" -days 1
  sudo tee "\$TMP/config" >/dev/null <<KC
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: \$SSL/kube-ca.pem
    server: https://127.0.0.1:6443
  name: cluster
contexts:
- context:
    cluster: cluster
    user: admin
  name: admin
current-context: admin
users:
- name: admin
  user:
    client-certificate: \$TMP/admin.crt
    client-key: \$TMP/admin.key
KC
fi
K="sudo docker run --rm --network host -v /etc/kubernetes/ssl:/etc/kubernetes/ssl -v /tmp/rke-admin-kubeconfig:/tmp/rke-admin-kubeconfig \$HYPERKUBE kubectl --kubeconfig /tmp/rke-admin-kubeconfig/config"


------







# --- 3. Pod-level DNS override (same hairpin fix inside agent containers) ---
# hostAliases applies inside cattle-cluster-agent and cattle-node-agent pods.
HOSTALIASES='[{"op":"replace","path":"/spec/template/spec/hostAliases","value":[{"ip":"'"\${RANCHER_PRIVATE}"'","hostnames":["'"\${RANCHER_HOST}"'"]}]}]'
\$K patch deployment cattle-cluster-agent -n cattle-system --type=json -p="\${HOSTALIASES}"
\$K patch daemonset cattle-node-agent -n cattle-system --type=json -p="\${HOSTALIASES}"




---





# --- 4. Update agent registration secret (stores base64 Rancher URL) ---
CRED=\$(\$K get deployment cattle-cluster-agent -n cattle-system -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="CATTLE_CREDENTIAL_NAME")].value}')
URL_B64=\$(echo -n "\${RANCHER_URL}" | base64)
\$K patch secret "\${CRED}" -n cattle-system --type=merge -p "{\"data\":{\"url\":\"\${URL_B64}\"}}"

---



# --- 5. Point agents at new Rancher + trust new CA ---
\$K set env deployment/cattle-cluster-agent -n cattle-system \\
  CATTLE_SERVER="\${RANCHER_URL}" CATTLE_CA_CHECKSUM="\${CATTLE_CA_CHECKSUM}"
\$K set env daemonset/cattle-node-agent -n cattle-system \\
  CATTLE_SERVER="\${RANCHER_URL}" CATTLE_CA_CHECKSUM="\${CATTLE_CA_CHECKSUM}"



----

# --- 6. Restart cluster-agent so pods pick up secret + env + hostAliases ---
\$K rollout restart deployment/cattle-cluster-agent -n cattle-system
\$K rollout status deployment/cattle-cluster-agent -n cattle-system --timeout=180s

echo "Done on \$(hostname)"
REMOTE

echo "=== Reconnect complete for node ${NODE_IP} ==="
