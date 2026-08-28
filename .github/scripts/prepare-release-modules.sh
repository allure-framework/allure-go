#!/usr/bin/env bash

# Prepare the modules in dependency order for a lockstep release.
#
# testify requires commons, so commons must be made available at the future
# version before testify can be tidied. The temporary file proxy supplies that
# version without publishing a tag or adding a replace directive.

set -euo pipefail

readonly script_name="${0##*/}"
repository_root="$(git rev-parse --show-toplevel)" || {
  printf '%s: run this command from a Git repository\n' "${script_name}" >&2
  exit 1
}
readonly repository_root
readonly modules_file="${repository_root}/.release-modules"
readonly temporary_parent="${RUNNER_TEMP:-/tmp}"

# Keep this list in dependency order. The release workflow also uses it to
# validate and tag every module.
release_modules=(commons testify)
readonly release_modules

release_version=""
release_temp_dir=""

fail() {
  printf '%s: %s\n' "${script_name}" "$*" >&2
  exit 1
}

validate_version() {
  if [ "$#" -ne 1 ]; then
    printf 'Usage: %s VERSION\n' "${script_name}" >&2
    printf 'Example: %s 1.3.0\n' "${script_name}" >&2
    exit 2
  fi
  if ! [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
    fail "version must be a Go semantic version without the v prefix"
  fi
  release_version="v$1"
}

cleanup() {
  if [ -n "${release_temp_dir}" ] && [ -d "${release_temp_dir}" ]; then
    rm -rf -- "${release_temp_dir}"
  fi
}

escape_proxy_component() {
  local value="$1"
  local escaped=""
  local character
  local index

  for ((index = 0; index < ${#value}; index++)); do
    character="${value:index:1}"
    if [[ "${character}" =~ [A-Z] ]]; then
      escaped="${escaped}!$(printf '%s' "${character}" | tr '[:upper:]' '[:lower:]')"
    else
      escaped="${escaped}${character}"
    fi
  done
  printf '%s' "${escaped}"
}

add_module_to_proxy() {
  local module_dir="$1"
  local module_path="$2"
  local escaped_path
  local escaped_version
  local proxy_version_dir
  local staging_dir
  local zip_root

  escaped_path="$(escape_proxy_component "${module_path}")"
  escaped_version="$(escape_proxy_component "${release_version}")"
  proxy_version_dir="${release_temp_dir}/proxy/${escaped_path}/@v"
  staging_dir="${release_temp_dir}/staging"
  zip_root="${module_path}@${release_version}"

  mkdir -p "${proxy_version_dir}" "${staging_dir}/${zip_root}"
  (
    cd "${module_dir}"
    git ls-files -z -- . | tar --null -T - -cf -
  ) | tar -xf - -C "${staging_dir}/${zip_root}"

  # Include manifests created or changed by tidy and the repository license
  # that Go adds to submodule archives.
  cp "${module_dir}/go.mod" "${staging_dir}/${zip_root}/go.mod"
  if [ -f "${module_dir}/go.sum" ]; then
    cp "${module_dir}/go.sum" "${staging_dir}/${zip_root}/go.sum"
  fi
  if [ ! -e "${staging_dir}/${zip_root}/LICENSE" ] && [ -f LICENSE ]; then
    cp LICENSE "${staging_dir}/${zip_root}/LICENSE"
  fi

  (
    cd "${staging_dir}"
    # Directory entries change the module hash, so omit them with -D.
    zip -q -X -D -r "${proxy_version_dir}/${escaped_version}.zip" "${zip_root}"
  )
  cp "${module_dir}/go.mod" "${proxy_version_dir}/${escaped_version}.mod"
  printf '{"Version":"%s","Time":"%s"}\n' \
    "${release_version}" \
    "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    > "${proxy_version_dir}/${escaped_version}.info"
}

main() {
  local module_dir
  local commons_path
  local release_proxy
  local upstream_proxy
  local internal_nosumdb

  validate_version "$@"
  cd "${repository_root}"

  for module_dir in "${release_modules[@]}"; do
    [ -f "${module_dir}/go.mod" ] || fail "missing module ${module_dir}"
  done
  printf '%s\n' "${release_modules[@]}" > "${modules_file}"
  printf 'Modules to release, in dependency order:\n'
  printf -- '- %s\n' "${release_modules[@]}"

  release_temp_dir="$(mktemp -d "${temporary_parent%/}/allure-go-release.XXXXXX")"
  trap cleanup EXIT

  commons_path="$(cd commons && GOWORK=off go list -m -f '{{ .Path }}')"
  (cd commons && GOWORK=off go mod tidy)
  add_module_to_proxy commons "${commons_path}"

  release_proxy="file://${release_temp_dir}/proxy"
  upstream_proxy="$(go env GOPROXY)"
  if [ -n "${upstream_proxy}" ] && [ "${upstream_proxy}" != "off" ]; then
    release_proxy="${release_proxy},${upstream_proxy}"
  fi

  internal_nosumdb="$(go env GONOSUMDB)"
  if [ -n "${internal_nosumdb}" ]; then
    internal_nosumdb="${internal_nosumdb},"
  fi
  internal_nosumdb="${internal_nosumdb}${commons_path}"

  (
    cd testify
    go mod edit -require="${commons_path}@${release_version}"
    GOWORK=off GOPROXY="${release_proxy}" GONOSUMDB="${internal_nosumdb}" go mod tidy
  )
}

main "$@"
