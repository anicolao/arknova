set -o errexit -o nounset -o pipefail

workshop_id=2630864749
json_file=""
output_dir="tts-assets"

usage() {
  cat <<'EOF'
Download every remote asset referenced by a Tabletop Simulator mod JSON.

Usage:
  tts-assets [--id WORKSHOP_ID] [--json FILE] [--output DIRECTORY]

Options:
  --id ID       Workshop ID (default: 2630864749)
  --json FILE   Explicit mod JSON; overrides automatic TTS path detection
  --output DIR  Destination directory (default: ./tts-assets)
  -h, --help    Show this help

On macOS the default JSON location is:
  ~/Library/Tabletop Simulator/Mods/Workshop/WORKSHOP_ID.json
EOF
}

while (( $# > 0 )); do
  case "$1" in
    --id)
      [[ $# -ge 2 ]] || { echo "--id requires a value" >&2; exit 2; }
      workshop_id=$2
      shift 2
      ;;
    --json)
      [[ $# -ge 2 ]] || { echo "--json requires a value" >&2; exit 2; }
      json_file=$2
      shift 2
      ;;
    --output)
      [[ $# -ge 2 ]] || { echo "--output requires a value" >&2; exit 2; }
      output_dir=$2
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ -z "$json_file" ]]; then
  case "$(uname -s)" in
    Darwin) tts_root="$HOME/Library/Tabletop Simulator" ;;
    Linux)  tts_root="$HOME/.local/share/Tabletop Simulator" ;;
    *)
      echo "Automatic TTS path detection is unsupported on this OS; use --json." >&2
      exit 2
      ;;
  esac
  json_file="$tts_root/Mods/Workshop/$workshop_id.json"
fi

if [[ ! -f "$json_file" ]]; then
  echo "TTS mod JSON not found: $json_file" >&2
  echo "Subscribe to and fully load the mod in TTS, or pass its file with --json." >&2
  exit 1
fi

mkdir -p "$output_dir/files"
urls_file="$output_dir/urls.txt"
manifest="$output_dir/manifest.tsv"

jq -r '.. | strings | select(test("^((\\{verifycache\\})?https?://)"))' "$json_file" \
  | sed 's/^{verifycache}//' \
  | LC_ALL=C sort -u > "$urls_file"

printf 'url\tfile\tmime_type\n' > "$manifest"

download_one() {
  local url=$1 hash tmp mime extension destination
  hash=$(printf '%s' "$url" | sha256sum | cut -d' ' -f1)
  tmp="$output_dir/files/.$hash.part"

  if ! curl --location --fail --retry 3 --retry-all-errors \
      --connect-timeout 20 --output "$tmp" "$url"; then
    rm -f "$tmp"
    printf '%s\t%s\t%s\n' "$url" "DOWNLOAD_FAILED" "" >> "$manifest"
    return 0
  fi

  mime=$(file --brief --mime-type "$tmp")
  case "$mime" in
    image/jpeg) extension=jpg ;;
    image/png) extension=png ;;
    image/webp) extension=webp ;;
    image/gif) extension=gif ;;
    application/pdf) extension=pdf ;;
    application/zip) extension=zip ;;
    application/json) extension=json ;;
    text/plain) extension=txt ;;
    model/obj) extension=obj ;;
    *) extension=bin ;;
  esac

  destination="$output_dir/files/$hash.$extension"
  mv -f "$tmp" "$destination"
  printf '%s\t%s\t%s\n' "$url" "files/$hash.$extension" "$mime" >> "$manifest"
}

url_count=$(wc -l < "$urls_file" | tr -d ' ')
echo "Found $url_count unique asset URLs in $json_file"

while IFS= read -r url; do
  [[ -n "$url" ]] && download_one "$url"
done < "$urls_file"

failures=$(awk -F '\t' '$2 == "DOWNLOAD_FAILED" { count++ } END { print count + 0 }' "$manifest")
echo "Assets:   $output_dir/files"
echo "Manifest: $manifest"
if (( failures > 0 )); then
  echo "Warning: $failures downloads failed; see the manifest." >&2
fi
