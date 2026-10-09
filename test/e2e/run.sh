#!/usr/bin/env bash
# End-to-end tests of provider-infisical. See test/e2e/README.md.
#
# Usage: test/e2e/run.sh <install|upgrade>
#
#   install  Install the locally built provider, then run the conversion tests
#            (TestConversion) and the lifecycle tests of every kind in both API
#            versions (TestLifecycle).
#   upgrade  Install the released provider (OLD_PROVIDER_PACKAGE), create every
#            kind as v1alpha1, upgrade in place to the locally built provider,
#            and check that the existing objects keep working (TestUpgrade).
#
# The script creates a kind cluster, installs Crossplane CROSSPLANE_VERSION
# and the provider, runs the Go tests in this folder, and deletes everything
# it created, also in Infisical.

set -euo pipefail

SUITE="${1:-}"
case "${SUITE}" in
  install|upgrade) ;;
  *) echo "usage: $0 <install|upgrade>" >&2; exit 2 ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CI="${GITHUB_ACTIONS:-false}"

# --- Settings ----------------------------------------------------------------

if [[ -n "${E2E_ENV_FILE:-}" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "${E2E_ENV_FILE}"
  set +a
fi

REQUIRED_ENV=(
  CROSSPLANE_VERSION PROVIDER_IMAGE PROVIDER_XPKG
  INFISICAL_HOST INFISICAL_CLIENT_ID INFISICAL_CLIENT_SECRET INFISICAL_ORG_ID INFISICAL_USER_EMAIL
  INFISICAL_GITHUB_CONNECTION_ID INFISICAL_GITHUB_REPO_OWNER INFISICAL_GITHUB_REPO_NAME
)
missing=()
for name in "${REQUIRED_ENV[@]}"; do
  [[ -n "${!name:-}" ]] || missing+=("${name}")
done
if (( ${#missing[@]} > 0 )); then
  message="missing required environment variables: ${missing[*]}. See test/e2e/README.md."
  if [[ "${CI}" == "true" ]]; then
    echo "::error title=E2E tests not configured::${message}"
  fi
  echo "ERROR: ${message}" >&2
  exit 1
fi

KIND="${KIND:-kind}"
HELM="${HELM:-helm}"
KUBECTL="${KUBECTL:-kubectl}"
CROSSPLANE_CLI="${CROSSPLANE_CLI:-crossplane}"
OLD_PROVIDER_PACKAGE="${OLD_PROVIDER_PACKAGE:-xpkg.upbound.io/infisical-inc/provider-infisical:v0.1.15}"
GOTESTSUM_VERSION="${GOTESTSUM_VERSION:-v1.13.0}"

# Every run has its own ID, which is part of the names of the Infisical
# resources, so that runs in parallel do not collide.
export E2E_RUN_ID="${E2E_RUN_ID:-$(od -An -N4 -tx1 /dev/urandom | tr -d ' \n')}"
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-e2e-${SUITE}-${CROSSPLANE_VERSION//./-}}"
ARTIFACTS_DIR="${E2E_ARTIFACTS_DIR:-${ROOT_DIR}/_output/e2e/${SUITE}-${CROSSPLANE_VERSION}}"
mkdir -p "${ARTIFACTS_DIR}"
WORK_DIR="$(mktemp -d)"
# Every run uses its own kubeconfig and API discovery cache, so that runs in
# parallel never act on the wrong cluster or see the APIs of an old cluster.
export KUBECONFIG="${WORK_DIR}/kubeconfig"
export KUBECACHEDIR="${WORK_DIR}/kube-cache"

NS="crossplane-system"
PROVIDER_NAME="provider-infisical"
RUNTIME_CONFIG_NAME="e2e-runtime"
# The same fake digest as build/makelib/local.xpkg.mk. With the pull policy
# Never, Crossplane reads the package from its cache instead of a registry.
LOCAL_DIGEST="sha256:0000000000000000000000000000000000000000000000000000000000000000"
LOCAL_PACKAGE="xpkg.crossplane.internal/dev/${PROVIDER_NAME}@${LOCAL_DIGEST}"
RUN_SELECTOR="e2e.infisical.com/run=${E2E_RUN_ID}"

# --- Output ------------------------------------------------------------------

log() { echo "[$(date +%H:%M:%S)] $*"; }
group() { if [[ "${CI}" == "true" ]]; then echo "::group::$*"; else log "== $*"; fi; }
endgroup() { if [[ "${CI}" == "true" ]]; then echo "::endgroup::"; fi; }
fail() {
  if [[ "${CI}" == "true" ]]; then echo "::error title=E2E ${SUITE} (Crossplane ${CROSSPLANE_VERSION})::$*"; fi
  log "FAIL: $*"
  exit 1
}

# --- Cluster and provider ----------------------------------------------------

create_cluster() {
  group "Create the kind cluster ${KIND_CLUSTER_NAME}"
  "${KIND}" delete cluster --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1 || true
  "${KIND}" create cluster --name "${KIND_CLUSTER_NAME}" --wait 120s
  endgroup
}

install_crossplane() {
  group "Install Crossplane ${CROSSPLANE_VERSION}"
  "${HELM}" repo add crossplane-stable https://charts.crossplane.io/stable --force-update >/dev/null
  "${HELM}" install crossplane crossplane-stable/crossplane --namespace "${NS}" --create-namespace \
    --version "${CROSSPLANE_VERSION}" --wait --timeout 5m
  endgroup
}

# Put the local package into the Crossplane package cache, the same way as
# build/makelib/local.xpkg.mk: a sidecar container shares the cache volume, and
# the package is copied in under both cache keys that Crossplane versions use.
load_local_package() {
  group "Load the local provider package and image"
  "${KUBECTL}" -n "${NS}" patch deployment/crossplane --type=json -p='[
    {"op":"add","path":"/spec/template/spec/containers/1","value":{"image":"alpine","name":"dev","command":["sleep","infinity"],"volumeMounts":[{"mountPath":"/tmp/cache","name":"package-cache"}]}},
    {"op":"add","path":"/spec/template/metadata/labels/patched","value":"true"}]'
  "${KUBECTL}" -n "${NS}" rollout status deployment/crossplane --timeout=180s

  local cache="${WORK_DIR}/cache" friendly pod
  mkdir -p "${cache}/xpkg.crossplane.internal/dev"
  "${CROSSPLANE_CLI}" xpkg extract --from-xpkg "${PROVIDER_XPKG}" -o "${cache}/xpkg.crossplane.internal/dev/${PROVIDER_NAME}@${LOCAL_DIGEST}.gz"
  friendly="$(printf '%.50s-%.12s' "xpkg.crossplane.internal/dev/${PROVIDER_NAME}" "${LOCAL_DIGEST}" | sed 's/[^a-z0-9]/-/g' | cut -c1-63 | sed 's/-*$//')"
  cp "${cache}/xpkg.crossplane.internal/dev/${PROVIDER_NAME}@${LOCAL_DIGEST}.gz" "${cache}/${friendly}.gz"
  pod="$("${KUBECTL}" -n "${NS}" get pod -l app=crossplane,patched=true --field-selector=status.phase=Running -o jsonpath='{.items[0].metadata.name}')"
  "${KUBECTL}" -n "${NS}" cp "${cache}" -c dev "${pod}:/tmp"
  "${KIND}" load docker-image "${PROVIDER_IMAGE}" --name "${KIND_CLUSTER_NAME}"

  # The readiness probe makes the provider ready only when its API conversion
  # webhook has started.
  "${KUBECTL}" apply -f - <<EOF
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
              ports:
                - name: readyz
                  containerPort: 8081
                  protocol: TCP
              readinessProbe:
                httpGet:
                  scheme: HTTP
                  port: readyz
                  path: /readyz
EOF
  endgroup
}

install_local_provider() {
  group "Install the provider under test"
  "${KUBECTL}" apply -f - <<EOF
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
  wait_provider "${LOCAL_PACKAGE}"
  endgroup
}

install_released_provider() {
  group "Install the released provider ${OLD_PROVIDER_PACKAGE}"
  "${KUBECTL}" apply -f - <<EOF
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: ${PROVIDER_NAME}
spec:
  package: ${OLD_PROVIDER_PACKAGE}
EOF
  wait_provider "${OLD_PROVIDER_PACKAGE}"
  endgroup
}

upgrade_provider() {
  group "Upgrade the provider in place to the provider under test"
  "${KUBECTL}" patch provider.pkg "${PROVIDER_NAME}" --type=merge \
    -p "{\"spec\":{\"package\":\"${LOCAL_PACKAGE}\",\"packagePullPolicy\":\"Never\",\"runtimeConfigRef\":{\"name\":\"${RUNTIME_CONFIG_NAME}\"}}}"
  wait_provider "${LOCAL_PACKAGE}"
  endgroup
}

# Crossplane v1 sets a "Healthy" condition on a provider revision. Crossplane
# v2 sets "RevisionHealthy" and "RuntimeHealthy" instead.
revision_healthy() {
  local conds
  conds="$("${KUBECTL}" get providerrevision "$1" -o jsonpath='{range .status.conditions[*]}{.type}={.status}{"\n"}{end}' 2>/dev/null)"
  grep -E -q '^(Healthy|RevisionHealthy)=True$' <<<"${conds}" && ! grep -E -q 'Healthy=(False|Unknown)$' <<<"${conds}"
}

# Wait until the active revision runs the given package, and its pod is the
# only ready provider pod. Then the old provider no longer reconciles.
wait_provider() {
  local package="$1" deadline=$((SECONDS + 600)) rev="" pods others
  log "waiting for the provider revision of ${package}"
  while (( SECONDS < deadline )); do
    rev="$("${KUBECTL}" get providerrevision -l "pkg.crossplane.io/package=${PROVIDER_NAME}" \
      -o jsonpath='{range .items[?(@.spec.desiredState=="Active")]}{.metadata.name} {.spec.image}{"\n"}{end}' 2>/dev/null \
      | awk -v p="${package}" '$2 == p {print $1}')"
    if [[ -n "${rev}" ]] && revision_healthy "${rev}"; then
      pods="$("${KUBECTL}" -n "${NS}" get pods -l "pkg.crossplane.io/provider=${PROVIDER_NAME}" -o jsonpath='{range .items[*]}{.metadata.labels.pkg\.crossplane\.io/revision}{"\n"}{end}')"
      others="$(grep -v -x "${rev}" <<<"${pods}" || true)"
      if grep -q -x "${rev}" <<<"${pods}" && [[ -z "${others}" ]] \
        && "${KUBECTL}" -n "${NS}" wait pod -l "pkg.crossplane.io/revision=${rev}" --for=condition=Ready --timeout=5s >/dev/null 2>&1; then
        "${KUBECTL}" wait provider.pkg "${PROVIDER_NAME}" --for=condition=Healthy --timeout=120s >/dev/null || fail "the provider is not healthy"
        log "provider revision ${rev} is active and its pod is ready"
        return 0
      fi
    fi
    sleep 5
  done
  fail "the provider ${package} did not become ready"
}

create_provider_config() {
  group "Create the ProviderConfig for ${INFISICAL_HOST}"
  local creds
  creds="$(jq -cn --arg id "${INFISICAL_CLIENT_ID}" --arg secret "${INFISICAL_CLIENT_SECRET}" '{auth:{universal:{client_id:$id,client_secret:$secret}}}')"
  "${KUBECTL}" -n "${NS}" create secret generic e2e-provider-creds --from-literal=credentials="${creds}" \
    --dry-run=client -o yaml | "${KUBECTL}" apply -f - >/dev/null
  "${KUBECTL}" apply -f - <<EOF
apiVersion: crossplane.infisical.com/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  host: ${INFISICAL_HOST}
  credentials:
    source: Secret
    secretRef:
      name: e2e-provider-creds
      namespace: ${NS}
      key: credentials
EOF
  endgroup
}

# --- Tests -------------------------------------------------------------------

# Run Go tests. The name is the name of the result file. The upgrade phase is
# for TestUpgrade.
run_go_tests() {
  local name="$1" pattern="$2" format="testname"
  export E2E_UPGRADE_PHASE="${3:-}"
  [[ "${CI}" == "true" ]] && format="github-actions"
  log "running ${pattern} (run ID ${E2E_RUN_ID})"
  (cd "${ROOT_DIR}" && go run "gotest.tools/gotestsum@${GOTESTSUM_VERSION}" \
    --format "${format}" \
    --jsonfile "${ARTIFACTS_DIR}/${name}.json" \
    --junitfile "${ARTIFACTS_DIR}/${name}.xml" \
    -- -tags e2e -count=1 -timeout 60m -run "${pattern}" ./test/e2e/)
}

# Write a summary of the test results for the GitHub job page.
write_summary() {
  [[ -n "${GITHUB_STEP_SUMMARY:-}" ]] || return 0
  local files
  files="$(ls "${ARTIFACTS_DIR}"/*.json 2>/dev/null || true)"
  [[ -n "${files}" ]] || return 0
  # The last pass, fail or skip event of every test is its result.
  # shellcheck disable=SC2086
  local results
  results="$(cat ${files} | jq -r -s '
    map(select(.Test != null and (.Action == "pass" or .Action == "fail" or .Action == "skip")))
    | group_by(.Test) | map(last) | sort_by(.Test) | .[] | "\(.Action)\t\(.Test)"')"
  {
    echo "### E2E ${SUITE}, Crossplane ${CROSSPLANE_VERSION}"
    echo
    echo "| Passed | Failed | Skipped |"
    echo "|---|---|---|"
    echo "| $(grep -c '^pass' <<<"${results}" || true) | $(grep -c '^fail' <<<"${results}" || true) | $(grep -c '^skip' <<<"${results}" || true) |"
    if grep -q '^fail' <<<"${results}"; then
      echo
      echo "**Failed tests**"
      echo
      grep '^fail' <<<"${results}" | cut -f2 | sed 's/^/- ❌ `/; s/$/`/'
    fi
    if grep -q '^skip' <<<"${results}"; then
      echo
      echo "<details><summary>Skipped tests</summary>"
      echo
      grep '^skip' <<<"${results}" | cut -f2 | sed 's/^/- `/; s/$/`/'
      echo
      echo "</details>"
    fi
    echo
  } >> "${GITHUB_STEP_SUMMARY}"
}

# --- Cleanup -----------------------------------------------------------------

dump_diagnostics() {
  log "saving diagnostics to ${ARTIFACTS_DIR}"
  "${KUBECTL}" get pkg,pkgrev -o wide > "${ARTIFACTS_DIR}/packages.txt" 2>&1 || true
  "${KUBECTL}" get managed -o yaml > "${ARTIFACTS_DIR}/managed-resources.yaml" 2>&1 || true
  "${KUBECTL}" -n "${NS}" get pods -o wide > "${ARTIFACTS_DIR}/pods.txt" 2>&1 || true
  "${KUBECTL}" -n "${NS}" logs -l "pkg.crossplane.io/provider=${PROVIDER_NAME}" --tail=-1 --all-containers > "${ARTIFACTS_DIR}/provider.log" 2>&1 || true
  "${KUBECTL}" -n "${NS}" logs deployment/crossplane -c crossplane --tail=-1 > "${ARTIFACTS_DIR}/crossplane.log" 2>&1 || true
  "${KUBECTL}" get events -A --sort-by=.lastTimestamp > "${ARTIFACTS_DIR}/events.txt" 2>&1 || true
}

# Delete the objects that the tests left, so that nothing stays in Infisical:
# first the objects that reference others, then the parents.
delete_leftovers() {
  local all parents children
  all="$("${KUBECTL}" get managed -l "${RUN_SELECTOR}" -o name 2>/dev/null || true)"
  [[ -n "${all}" ]] || return 0
  log "deleting the objects that the tests left"
  parents="$(grep -E '^(project\.project|identity\.identity|group\.group)\.' <<<"${all}" || true)"
  children="$(grep -E -v '^(project\.project|identity\.identity|group\.group)\.' <<<"${all}" || true)"
  # shellcheck disable=SC2086
  [[ -z "${children}" ]] || "${KUBECTL}" delete ${children} --wait=true --timeout=180s >/dev/null 2>&1 || true
  # shellcheck disable=SC2086
  [[ -z "${parents}" ]] || "${KUBECTL}" delete ${parents} --wait=true --timeout=180s >/dev/null 2>&1 || true
  all="$("${KUBECTL}" get managed -l "${RUN_SELECTOR}" -o name 2>/dev/null || true)"
  if [[ -n "${all}" ]]; then
    local message="some test objects were not deleted. Delete the Infisical resources with \"${E2E_RUN_ID}\" in their name by hand: $(tr '\n' ' ' <<<"${all}")"
    if [[ "${CI}" == "true" ]]; then echo "::warning title=E2E cleanup::${message}"; fi
    log "WARNING: ${message}"
  fi
}

cleanup() {
  local code=$?
  set +e
  if [[ -s "${KUBECONFIG}" ]]; then
    (( code == 0 )) || dump_diagnostics
    delete_leftovers
  fi
  write_summary
  if [[ "${KEEP_CLUSTER:-false}" != "true" ]]; then
    "${KIND}" delete cluster --name "${KIND_CLUSTER_NAME}" >/dev/null 2>&1
  fi
  rm -rf "${WORK_DIR}"
  exit "${code}"
}
trap cleanup EXIT

# --- Main --------------------------------------------------------------------

log "E2E ${SUITE} suite, Crossplane ${CROSSPLANE_VERSION}, run ID ${E2E_RUN_ID}"
create_cluster
install_crossplane
load_local_package
if [[ "${SUITE}" == "install" ]]; then
  install_local_provider
  create_provider_config
  run_go_tests install '^(TestConversion|TestLifecycle)$'
else
  install_released_provider
  create_provider_config
  run_go_tests upgrade-before '^TestUpgrade$' before
  upgrade_provider
  run_go_tests upgrade-after '^TestUpgrade$' after
fi
log "PASS"
