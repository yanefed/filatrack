#!/usr/bin/env bash
set -euo pipefail

: "${FILATRACK_BUILD_MANAGED:?Run this script through: go run ./cmd/build flatpak}"
: "${FILATRACK_APP_ID:?Missing application metadata}"
: "${FILATRACK_APP_NAME:?Missing application metadata}"
: "${FILATRACK_VERSION:?Missing application metadata}"
: "${FILATRACK_FLATPAK_RUNTIME_VERSION:?Missing application metadata}"
: "${FILATRACK_GOLANG_SDK_VERSION:?Missing application metadata}"

project_root="$(cd "$(dirname "$0")/../.." && pwd -P)"
app_id="$FILATRACK_APP_ID"
manifest="$project_root/filatrack.flatpak.yml"
runtime_version="$FILATRACK_FLATPAK_RUNTIME_VERSION"
golang_sdk_version="$FILATRACK_GOLANG_SDK_VERSION"

if command -v flatpak-builder >/dev/null 2>&1; then
  builder=(flatpak-builder)
elif flatpak info --user org.flatpak.Builder >/dev/null 2>&1; then
  builder=(flatpak run --env="FLATPAK_USER_DIR=$HOME/.local/share/flatpak" --command=flatpak-builder org.flatpak.Builder)
else
  echo "Flatpak Builder is required. Install it with:" >&2
  echo "  flatpak install --user flathub org.flatpak.Builder" >&2
  exit 1
fi

flatpak install --user --noninteractive flathub \
  "org.gnome.Platform//$runtime_version" \
  "org.gnome.Sdk//$runtime_version" \
  "org.freedesktop.Sdk.Extension.golang//$golang_sdk_version"

cd "$project_root"
go mod vendor
cleanup() {
  rm -rf -- vendor
}
trap cleanup EXIT

"${builder[@]}" --user --install --force-clean build-dir "$manifest"
bundle="$project_root/dist/$FILATRACK_APP_NAME-$FILATRACK_VERSION.flatpak"
mkdir -p "$project_root/dist"
flatpak build-bundle ~/.local/share/flatpak/repo "$bundle" "$app_id"

echo "Built $bundle"
