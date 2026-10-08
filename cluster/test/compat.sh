#!/usr/bin/env bash
# Crossplane compatibility test for provider-infisical.
#
# This script creates a kind cluster, installs the given Crossplane version,
# installs the provider and reconciles one managed resource (MR) of every kind.
#
# Usage: cluster/test/compat.sh <fresh|upgrade>
#
#   fresh    Install the locally built provider package and test it.
#   upgrade  Install the released provider (OLD_PROVIDER_PACKAGE), create the
#            MRs, then upgrade in place to the locally built package. The test
#            checks that the new provider takes over the existing MRs, keeps
#            their external names (no resource is recreated) and can update and
#            delete them.
#
# Required environment:
#   CROSSPLANE_VERSION     Crossplane Helm chart version, for example 1.20.0
#   PROVIDER_IMAGE         Local docker image of the provider (from make build)
#   PROVIDER_XPKG          Path to the locally built .xpkg (from make build)
#
# Optional environment:
#   OLD_PROVIDER_PACKAGE   Released package for the upgrade test
#   KIND_CLUSTER_NAME      Name of the kind cluster (default infisical-compat)
#   KIND_NODE_IMAGE        kind node image (default: the kind default)
#   KEEP_CLUSTER           "true" keeps the cluster after the test
#   REUSE_CLUSTER          "true" uses an existing cluster with Crossplane
#                          installed (for debugging the fresh mode)
#   COMPAT_ENV_FILE        File with KEY=VALUE lines that is sourced first
#   KIND, HELM, KUBECTL, CROSSPLANE_CLI   Paths to the tools
#
# Live mode. When all of these are set, the MRs are created in a real Infisical
# instance and the "ready" tier MRs must become Ready=True. When they are not
# set, the provider points to an address that does not answer, and the test
# only checks that every controller reconciles its MRs.
#   INFISICAL_HOST           URL that the kind cluster can reach
#   INFISICAL_CLIENT_ID      Universal auth client ID of an org admin identity
#   INFISICAL_CLIENT_SECRET  Universal auth client secret
#   INFISICAL_ORG_ID         Organization ID
#   INFISICAL_USER_EMAIL     Email of a user in the organization
#
# Optional for live mode. When all are set, the GitHub secret sync must also
# become Ready. The sync never deletes secrets in the repository.
#   INFISICAL_GITHUB_CONNECTION_ID  ID of a GitHub App connection in the org
#   INFISICAL_GITHUB_REPO_OWNER     Owner of the repository to sync to
#   INFISICAL_GITHUB_REPO_NAME      Name of the repository to sync to

set -euo pipefail

MODE="${1:-fresh}"
case "${MODE}" in
  fresh|upgrade) ;;
  *) echo "usage: $0 <fresh|upgrade>" >&2; exit 2 ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST_DIR="${ROOT_DIR}/cluster/test/compat"

if [[ -n "${COMPAT_ENV_FILE:-}" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "${COMPAT_ENV_FILE}"
  set +a
fi

: "${CROSSPLANE_VERSION:?CROSSPLANE_VERSION is required}"
: "${PROVIDER_IMAGE:?PROVIDER_IMAGE is required}"
: "${PROVIDER_XPKG:?PROVIDER_XPKG is required}"

KIND="${KIND:-kind}"
HELM="${HELM:-helm}"
KUBECTL="${KUBECTL:-kubectl}"
CROSSPLANE_CLI="${CROSSPLANE_CLI:-crossplane}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-infisical-compat}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"
REUSE_CLUSTER="${REUSE_CLUSTER:-false}"
OLD_PROVIDER_PACKAGE="${OLD_PROVIDER_PACKAGE:-xpkg.upbound.io/infisical-inc/provider-infisical:v0.1.15}"

NS="crossplane-system"
PROVIDER_NAME="provider-infisical"
RUNTIME_CONFIG_NAME="compat-runtime"
# The same fake digest as build/makelib/local.xpkg.mk. With pull policy Never,
# Crossplane reads the package from its cache instead of a registry.
LOCAL_DIGEST="sha256:0000000000000000000000000000000000000000000000000000000000000000"
LOCAL_PACKAGE="xpkg.crossplane.internal/dev/${PROVIDER_NAME}@${LOCAL_DIGEST}"
TEST_SELECTOR="compat.infisical.com/test=true"
READY_TIMEOUT="${READY_TIMEOUT:-600}"
# Unique for every run, also for runs that start in the same second.
RUN_ID="$(od -An -N5 -tx1 /dev/urandom | tr -d ' \n')"
WORK_DIR="$(mktemp -d)"
# Every run uses its own kubeconfig, so that runs in parallel never act on the
# wrong cluster through a shared current context.
export KUBECONFIG="${WORK_DIR}/kubeconfig"

if [[ -n "${INFISICAL_HOST:-}" && -n "${INFISICAL_CLIENT_ID:-}" && -n "${INFISICAL_CLIENT_SECRET:-}" && -n "${INFISICAL_ORG_ID:-}" ]]; then
  LIVE=true
else
  LIVE=false
fi

if [[ "${LIVE}" == "true" && -n "${INFISICAL_GITHUB_CONNECTION_ID:-}" && -n "${INFISICAL_GITHUB_REPO_OWNER:-}" && -n "${INFISICAL_GITHUB_REPO_NAME:-}" ]]; then
  GITHUB_SYNC_TIER="ready"
else
  GITHUB_SYNC_TIER="synced"
fi

log() { echo "[$(date +%H:%M:%S)] [${CROSSPLANE_VERSION}/${MODE}] $*"; }

dump_diagnostics() {
  log "diagnostics"
  "${KUBECTL}" get pkg,pkgrev -o wide || true
  "${KUBECTL}" get managed -o wide || true
  "${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o yaml > "${WORK_DIR}/managed.yaml" 2>/dev/null || true
  "${KUBECTL}" -n "${NS}" get pods -o wide || true
  "${KUBECTL}" -n "${NS}" logs -l "pkg.crossplane.io/provider=${PROVIDER_NAME}" --tail=200 --all-containers || true
  "${KUBECTL}" get events -A --sort-by=.lastTimestamp | tail -n 60 || true
  log "MR manifests saved to ${WORK_DIR}/managed.yaml"
}

fail() {
  log "FAIL: $*"
  dump_diagnostics
  exit 1
}

# Remove the test MRs, so that live runs do not leave resources behind in the
# Infisical instance. Then remove the cluster.
cleanup() {
  local code=$?
  set +e
  if [[ -s "${KUBECONFIG}" ]]; then
    if [[ "${LIVE}" == "true" ]] && [[ -n "$("${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o name 2>/dev/null)" ]]; then
      log "cleanup: deleting test MRs"
      delete_test_mrs 180 \
        || log "cleanup: WARNING: some MRs were not deleted. Check the Infisical instance for resources with suffix ${RUN_ID}."
    fi
  fi
  if [[ "${KEEP_CLUSTER}" != "true" ]]; then
    "${KIND}" delete cluster --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1
  fi
  exit "${code}"
}
trap cleanup EXIT

create_cluster() {
  log "creating kind cluster ${KIND_CLUSTER_NAME}"
  "${KIND}" delete cluster --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1 || true
  local args=(create cluster --name "${KIND_CLUSTER_NAME}" --wait 120s)
  if [[ -n "${KIND_NODE_IMAGE}" ]]; then
    args+=(--image "${KIND_NODE_IMAGE}")
  fi
  "${KIND}" "${args[@]}"
}

install_crossplane() {
  log "installing Crossplane ${CROSSPLANE_VERSION}"
  "${HELM}" repo add crossplane-stable https://charts.crossplane.io/stable --force-update >/dev/null
  "${HELM}" install crossplane crossplane-stable/crossplane \
    --namespace "${NS}" --create-namespace \
    --version "${CROSSPLANE_VERSION}" --wait --timeout 5m >/dev/null
}

# Put the local package into the Crossplane package cache. This is the same
# method as build/makelib/local.xpkg.mk: a sidecar container shares the cache
# volume, and the package is copied in under both cache keys that Crossplane
# versions use.
load_local_package() {
  log "loading the local provider package and image"
  if ! "${KUBECTL}" -n "${NS}" get deployment crossplane -o jsonpath='{.spec.template.spec.containers[*].name}' | grep -q dev; then
    "${KUBECTL}" -n "${NS}" patch deployment/crossplane --type=json -p='[
      {"op":"add","path":"/spec/template/spec/containers/1","value":{"image":"alpine","name":"dev","command":["sleep","infinity"],"volumeMounts":[{"mountPath":"/tmp/cache","name":"package-cache"}]}},
      {"op":"add","path":"/spec/template/metadata/labels/patched","value":"true"}]' >/dev/null
    "${KUBECTL}" -n "${NS}" rollout status deployment/crossplane --timeout=180s >/dev/null
  fi

  local cache="${WORK_DIR}/cache"
  mkdir -p "${cache}/xpkg.crossplane.internal/dev"
  "${CROSSPLANE_CLI}" xpkg extract --from-xpkg "${PROVIDER_XPKG}" -o "${cache}/xpkg.crossplane.internal/dev/${PROVIDER_NAME}@${LOCAL_DIGEST}.gz"
  local friendly
  friendly="$(printf '%.50s-%.12s' "xpkg.crossplane.internal/dev/${PROVIDER_NAME}" "${LOCAL_DIGEST}" | sed 's/[^a-z0-9]/-/g' | cut -c1-63 | sed 's/-*$//')"
  cp "${cache}/xpkg.crossplane.internal/dev/${PROVIDER_NAME}@${LOCAL_DIGEST}.gz" "${cache}/${friendly}.gz"

  local pod
  pod="$("${KUBECTL}" -n "${NS}" get pod -l app=crossplane,patched=true --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}')"
  "${KUBECTL}" -n "${NS}" cp "${cache}" -c dev "${pod}:/tmp"

  "${KIND}" load docker-image "${PROVIDER_IMAGE}" --name "${KIND_CLUSTER_NAME}" >/dev/null

  "${KUBECTL}" apply -f - >/dev/null <<EOF
apiVersion: pkg.crossplane.io/v1beta1
kind: DeploymentRuntimeConfig
metadata:
  name: ${RUNTIME_CONFIG_NAME}
spec:
  deploymentTemplate:
    spec:
      selector: {}
      template:
        spec:
          containers:
            - name: package-runtime
              image: ${PROVIDER_IMAGE}
              imagePullPolicy: Never
              args: ["--debug", "--poll=1m"]
EOF
}

install_local_provider() {
  log "installing the local provider package"
  "${KUBECTL}" apply -f - >/dev/null <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: ${PROVIDER_NAME}
spec:
  package: ${LOCAL_PACKAGE}
  packagePullPolicy: Never
  runtimeConfigRef:
    name: ${RUNTIME_CONFIG_NAME}
EOF
}

install_old_provider() {
  log "installing the released provider ${OLD_PROVIDER_PACKAGE}"
  "${KUBECTL}" apply -f - >/dev/null <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: ${PROVIDER_NAME}
spec:
  package: ${OLD_PROVIDER_PACKAGE}
EOF
}

upgrade_to_local_provider() {
  log "upgrading the provider in place to the local package"
  "${KUBECTL}" patch provider.pkg "${PROVIDER_NAME}" --type=merge -p "{\"spec\":{\"package\":\"${LOCAL_PACKAGE}\",\"packagePullPolicy\":\"Never\",\"runtimeConfigRef\":{\"name\":\"${RUNTIME_CONFIG_NAME}\"}}}" >/dev/null
}

# Crossplane v1 sets a "Healthy" condition on a provider revision. Crossplane
# v2 sets "RevisionHealthy" and "RuntimeHealthy" instead. The revision is
# healthy when it has at least one of these and none of them is False.
revision_healthy() {
  local conds
  conds="$("${KUBECTL}" get providerrevision "$1" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null)"
  grep -E -q '^(Healthy|RevisionHealthy)=True$' <<<"${conds}" && ! grep -E -q 'Healthy=(False|Unknown)$' <<<"${conds}"
}

# Wait until the active revision runs the given package and its pod is the only
# provider pod. This makes sure that the old provider no longer reconciles.
wait_provider() {
  local package="$1"
  log "waiting for the provider revision of ${package}"
  local deadline=$((SECONDS + 600)) rev=""
  while (( SECONDS < deadline )); do
    rev="$("${KUBECTL}" get providerrevision -l "pkg.crossplane.io/package=${PROVIDER_NAME}" \
      -o jsonpath="{range .items[?(@.spec.desiredState==\"Active\")]}{.metadata.name} {.spec.image}{\"\n\"}{end}" 2>/dev/null \
      | awk -v p="${package}" '$2 == p {print $1}')"
    if [[ -n "${rev}" ]] && revision_healthy "${rev}"; then
      break
    fi
    sleep 5
  done
  [[ -n "${rev}" ]] || fail "no active provider revision for ${package}"
  revision_healthy "${rev}" || fail "provider revision ${rev} is not healthy"
  "${KUBECTL}" wait provider.pkg "${PROVIDER_NAME}" --for=condition=Healthy --timeout=300s >/dev/null || fail "provider is not healthy"

  while (( SECONDS < deadline )); do
    local pods other
    pods="$("${KUBECTL}" -n "${NS}" get pods -l "pkg.crossplane.io/provider=${PROVIDER_NAME}" -o jsonpath='{range .items[*]}{.metadata.labels.pkg\.crossplane\.io/revision}{"\n"}{end}')"
    other="$(grep -v -x "${rev}" <<<"${pods}" || true)"
    if grep -q -x "${rev}" <<<"${pods}" && [[ -z "${other}" ]] \
      && "${KUBECTL}" -n "${NS}" wait pod -l "pkg.crossplane.io/revision=${rev}" --for=condition=Ready --timeout=5s >/dev/null 2>&1; then
      log "provider revision ${rev} is active and its pod is ready"
      CURRENT_REVISION="${rev}"
      return 0
    fi
    sleep 5
  done
  fail "the provider pod of revision ${rev} did not become the only ready provider pod"
}

check_crds() {
  log "checking that all provider CRDs are established"
  local crds
  crds="$("${KUBECTL}" get crd -o name | grep 'crossplane\.infisical\.com$' || true)"
  [[ -n "${crds}" ]] || fail "no provider CRDs found"
  # shellcheck disable=SC2086
  "${KUBECTL}" wait --for=condition=Established --timeout=120s ${crds} >/dev/null || fail "not all CRDs are established"
  log "$(wc -l <<<"${crds}" | tr -d ' ') CRDs are established"
}

create_provider_config() {
  local host client_id client_secret
  if [[ "${LIVE}" == "true" ]]; then
    host="${INFISICAL_HOST}"
    client_id="${INFISICAL_CLIENT_ID}"
    client_secret="${INFISICAL_CLIENT_SECRET}"
    log "creating the ProviderConfig (live mode, host ${host})"
  else
    # Nothing listens on this address, so every external call fails fast.
    host="http://127.0.0.1:1"
    client_id="compat"
    client_secret="compat"
    log "creating the ProviderConfig (offline mode)"
  fi
  local creds
  creds="$(jq -cn --arg id "${client_id}" --arg secret "${client_secret}" '{auth:{universal:{client_id:$id,client_secret:$secret}}}')"
  "${KUBECTL}" -n "${NS}" create secret generic compat-provider-creds --from-literal=credentials="${creds}" \
    --dry-run=client -o yaml | "${KUBECTL}" apply -f - >/dev/null
  "${KUBECTL}" apply -f - >/dev/null <<EOF
apiVersion: crossplane.infisical.com/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  host: ${host}
  credentials:
    source: Secret
    secretRef:
      name: compat-provider-creds
      namespace: ${NS}
      key: credentials
EOF
}

render() {
  sed -e "s|__RUN_ID__|${RUN_ID}|g" \
    -e "s|__ORG_ID__|${INFISICAL_ORG_ID:-00000000-0000-0000-0000-000000000000}|g" \
    -e "s|__USER_EMAIL__|${INFISICAL_USER_EMAIL:-compat@example.com}|g" \
    -e "s|__PROJECT_ID__|${1:-00000000-0000-0000-0000-000000000000}|g" \
    -e "s|__GITHUB_SYNC_TIER__|${GITHUB_SYNC_TIER}|g" \
    -e "s|__GITHUB_CONNECTION_ID__|${INFISICAL_GITHUB_CONNECTION_ID:-00000000-0000-0000-0000-000000000000}|g" \
    -e "s|__GITHUB_REPO_OWNER__|${INFISICAL_GITHUB_REPO_OWNER:-compat}|g" \
    -e "s|__GITHUB_REPO_NAME__|${INFISICAL_GITHUB_REPO_NAME:-compat}|g"
}

apply_resources() {
  log "applying test MRs (run ID ${RUN_ID})"
  # Old CRDs may still be in the API discovery cache right after install.
  local i
  for i in 1 2 3 4 5 6; do
    if render < "${TEST_DIR}/resources.yaml" | "${KUBECTL}" apply -f - >/dev/null; then
      break
    fi
    (( i < 6 )) || fail "cannot apply the test MRs"
    sleep 10
  done

  local project_id=""
  if [[ "${LIVE}" == "true" ]]; then
    "${KUBECTL}" wait project.project.crossplane.infisical.com/compat-project --for=condition=Ready --timeout="${READY_TIMEOUT}s" >/dev/null \
      || fail "the Project did not become ready"
    project_id="$("${KUBECTL}" get project.project.crossplane.infisical.com/compat-project -o jsonpath='{.status.atProvider.id}')"
  fi
  render "${project_id}" < "${TEST_DIR}/secret.yaml" | "${KUBECTL}" apply -f - >/dev/null || fail "cannot apply the Secret MR"
}

# Print "<resource> <tier>" for every test MR.
list_mrs() {
  local mr
  for mr in $("${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o name); do
    echo "${mr} $("${KUBECTL}" get "${mr}" -o jsonpath='{.metadata.labels.compat\.infisical\.com/tier}')"
  done
}

# The provider revision that wait_provider found active.
CURRENT_REVISION=""

# Log the pending MRs at most once a minute.
LAST_PENDING_LOG=0
log_pending() {
  if (( SECONDS - LAST_PENDING_LOG >= 60 )); then
    log "still waiting for: $1"
    LAST_PENDING_LOG=${SECONDS}
  fi
}

condition() {
  "${KUBECTL}" get "$1" -o jsonpath="{.status.conditions[?(@.type==\"$2\")].status}" 2>/dev/null
}

# Wait until every MR is reconciled. In live mode the "ready" tier must become
# Ready=True and Synced=True. Otherwise the MR only needs a Synced condition.
wait_mrs() {
  log "waiting for the test MRs"
  local deadline=$((SECONDS + READY_TIMEOUT)) pending
  while true; do
    pending=""
    while read -r mr tier; do
      [[ -n "${mr}" ]] || continue
      if [[ "${LIVE}" == "true" && "${tier}" == "ready" ]]; then
        [[ "$(condition "${mr}" Ready)" == "True" && "$(condition "${mr}" Synced)" == "True" ]] || pending+="${mr} "
      else
        [[ -n "$(condition "${mr}" Synced)" ]] || pending+="${mr} "
      fi
    done < <(list_mrs)
    [[ -z "${pending}" ]] && break
    (( SECONDS < deadline )) || fail "MRs not reconciled in time: ${pending}"
    log_pending "${pending}"
    sleep 10
  done
  log "all $(list_mrs | wc -l | tr -d ' ') test MRs are reconciled"
}

# Check that the running provider pod reconciles every test MR, and that the
# MRs keep their state. The provider runs with --debug and logs each
# reconcile. Only the pod of CURRENT_REVISION is checked, so after an upgrade
# this proves that the new provider handled the existing MRs.
#
# A completed reconcile of a "ready" MR logs "External resource is up to date".
# MRs in the "synced" tier only need a "Reconciling" line. In live mode the
# "ready" MRs must still be Ready=True and Synced=True after the reconcile.
check_reconciled() {
  local since
  since="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  log "checking that provider revision ${CURRENT_REVISION} reconciles every test MR"
  # Any change to an MR triggers a reconcile.
  "${KUBECTL}" annotate managed -l "${TEST_SELECTOR}" "compat.infisical.com/reconcile=${since}" --overwrite >/dev/null
  local deadline=$((SECONDS + READY_TIMEOUT)) pending logs kind name pattern
  while true; do
    pending=""
    logs="$("${KUBECTL}" -n "${NS}" logs -l "pkg.crossplane.io/revision=${CURRENT_REVISION}" --tail=-1 --since-time="${since}" 2>/dev/null)"
    while read -r mr tier; do
      [[ -n "${mr}" ]] || continue
      kind="${mr%%.*}"
      name="${mr#*/}"
      if [[ "${LIVE}" == "true" && "${tier}" == "ready" ]]; then
        pattern="External resource is up to date"
      else
        pattern="Reconciling"
      fi
      if ! grep -F "${pattern}" <<<"${logs}" | grep -F "kind=${kind}\"" | grep -F -q "\"request\": {\"name\":\"${name}\"}"; then
        pending+="${mr} "
        continue
      fi
      if [[ "${LIVE}" == "true" && "${tier}" == "ready" ]]; then
        [[ "$(condition "${mr}" Ready)" == "True" && "$(condition "${mr}" Synced)" == "True" ]] || pending+="${mr} "
      fi
    done < <(list_mrs)
    [[ -z "${pending}" ]] && break
    (( SECONDS < deadline )) || fail "MRs not reconciled by revision ${CURRENT_REVISION} in time: ${pending}"
    log_pending "${pending}"
    sleep 10
  done
  log "provider revision ${CURRENT_REVISION} reconciled every test MR"
}

# Save "<resource> <external name> <id>" of the ready tier, so that the upgrade
# test can check that no resource was recreated.
snapshot_external_names() {
  local mr tier
  while read -r mr tier; do
    [[ "${tier}" == "ready" ]] || continue
    echo "${mr} $("${KUBECTL}" get "${mr}" -o jsonpath='{.metadata.annotations.crossplane\.io/external-name} {.status.atProvider.id}')"
  done < <(list_mrs) | sort
}

check_update() {
  local description
  description="updated by compat test ${RUN_ID}"
  log "updating the Project description"
  "${KUBECTL}" patch project.project.crossplane.infisical.com/compat-project --type=merge \
    -p "{\"spec\":{\"forProvider\":{\"description\":\"${description}\"}}}" >/dev/null
  local deadline=$((SECONDS + READY_TIMEOUT))
  until [[ "$("${KUBECTL}" get project.project.crossplane.infisical.com/compat-project -o jsonpath='{.status.atProvider.description}')" == "${description}" ]]; do
    (( SECONDS < deadline )) || fail "the Project update was not applied"
    sleep 5
  done
  [[ "$(condition project.project.crossplane.infisical.com/compat-project Synced)" == "True" ]] || fail "the Project is not synced after the update"
  log "the Project update was applied in Infisical"
}

# Delete the test MRs in two steps: first the MRs that depend on other MRs,
# then the parents. When a parent goes first, Infisical deletes the children
# with it, and some Terraform resources fail to read a child whose parent is
# gone, so their MRs never finish deleting.
PARENT_KINDS_REGEX='^(project\.project|identity\.identity|group\.group)\.crossplane\.infisical\.com/'

delete_test_mrs() {
  local timeout="$1" mr children parents
  # MRs in the "synced" tier were never created externally, and some of them
  # cannot observe an empty ID. With the Orphan policy the provider removes
  # the finalizer without an external call.
  for mr in $("${KUBECTL}" get managed -l "${TEST_SELECTOR},compat.infisical.com/tier=synced" -o name); do
    "${KUBECTL}" patch "${mr}" --type=merge -p '{"spec":{"deletionPolicy":"Orphan"}}' >/dev/null
  done
  children="$("${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o name | grep -E -v "${PARENT_KINDS_REGEX}" || true)"
  parents="$("${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o name | grep -E "${PARENT_KINDS_REGEX}" || true)"
  for mr in ${children}; do "${KUBECTL}" delete "${mr}" --wait=false >/dev/null; done
  for mr in ${children}; do "${KUBECTL}" wait --for=delete "${mr}" --timeout="${timeout}s" >/dev/null 2>&1 || return 1; done
  for mr in ${parents}; do "${KUBECTL}" delete "${mr}" --wait=false >/dev/null; done
  for mr in ${parents}; do "${KUBECTL}" wait --for=delete "${mr}" --timeout="${timeout}s" >/dev/null 2>&1 || return 1; done
}

check_delete() {
  log "deleting the test MRs"
  if [[ "${LIVE}" != "true" ]]; then
    # Nothing exists externally in offline mode. With the Orphan policy the
    # provider removes the finalizer without an external call.
    local mr
    for mr in $("${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o name); do
      "${KUBECTL}" patch "${mr}" --type=merge -p '{"spec":{"deletionPolicy":"Orphan"}}' >/dev/null
    done
  fi
  delete_test_mrs "${READY_TIMEOUT}" \
    || fail "MRs not deleted in time: $("${KUBECTL}" get managed -l "${TEST_SELECTOR}" -o name | tr '\n' ' ')"
  log "all test MRs are deleted"
}

check_provider_pod() {
  local restarts
  restarts="$("${KUBECTL}" -n "${NS}" get pods -l "pkg.crossplane.io/provider=${PROVIDER_NAME}" -o jsonpath='{range .items[*]}{.status.containerStatuses[*].restartCount}{"\n"}{end}' | awk '{s+=$1} END {print s+0}')"
  [[ "${restarts}" == "0" ]] || fail "the provider pod restarted ${restarts} time(s)"
  if "${KUBECTL}" -n "${NS}" logs -l "pkg.crossplane.io/provider=${PROVIDER_NAME}" --tail=-1 | grep -E -q '^panic:|goroutine [0-9]+ \[running\]'; then
    fail "the provider logged a panic"
  fi
}

main() {
  log "start (live mode: ${LIVE}, GitHub sync tier: ${GITHUB_SYNC_TIER})"
  if [[ "${REUSE_CLUSTER}" == "true" ]] && "${KIND}" get clusters 2>/dev/null | grep -q -x "${KIND_CLUSTER_NAME}"; then
    log "reusing kind cluster ${KIND_CLUSTER_NAME}"
    "${KIND}" export kubeconfig --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1
  else
    create_cluster
    install_crossplane
  fi
  load_local_package

  if [[ "${MODE}" == "upgrade" ]]; then
    install_old_provider
    wait_provider "${OLD_PROVIDER_PACKAGE}"
  else
    install_local_provider
    wait_provider "${LOCAL_PACKAGE}"
  fi
  check_crds
  create_provider_config
  apply_resources
  wait_mrs

  if [[ "${MODE}" == "upgrade" ]]; then
    local before after
    before="$(snapshot_external_names)"
    upgrade_to_local_provider
    wait_provider "${LOCAL_PACKAGE}"
    check_crds
    check_reconciled
    after="$(snapshot_external_names)"
    if [[ "${before}" != "${after}" ]]; then
      echo "before upgrade:"; echo "${before}"
      echo "after upgrade:"; echo "${after}"
      fail "external names or IDs changed during the upgrade"
    fi
    log "external names and IDs did not change during the upgrade"
  else
    check_reconciled
  fi

  if [[ "${LIVE}" == "true" ]]; then
    check_update
  fi
  check_provider_pod
  check_delete
  check_provider_pod
  log "PASS"
}

main
