#!/usr/bin/env bash
set -euo pipefail

app_id=com.filatrack.app
manifest="$app_id.yml"

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
  org.gnome.Platform//50 \
  org.gnome.Sdk//50 \
  org.freedesktop.Sdk.Extension.golang//25.08

go mod vendor
cleanup() {
  rm -rf -- vendor
}
trap cleanup EXIT

"${builder[@]}" --user --install --force-clean build-dir "$manifest"
flatpak build-bundle ~/.local/share/flatpak/repo "$app_id.flatpak" "$app_id"

echo "Built $app_id.flatpak"
