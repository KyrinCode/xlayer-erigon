#! /bin/sh


if ! command -v yq >/dev/null 2>&1; then
  sudo apt update && sudo apt install -y yq
fi

CONFIG_FILES=(./test/config/test.erigon.seq.config.yaml)

for CONFIG_FILE in "${CONFIG_FILES[@]}"; do
#  cat "$CONFIG_FILE"
  echo "Updating $CONFIG_FILE..."

  yq -i '.zkevm.executor-urls = "xlayer-executor:50071"' "$CONFIG_FILE"
  yq -i '.zkevm.executor-strict = true' "$CONFIG_FILE"
  yq -i '.zkevm.witness-full = false' "$CONFIG_FILE"
  yq -i '.zkevm.executor-mock = false' "$CONFIG_FILE"

  echo "Finished updating $CONFIG_FILE."
#  cat "$CONFIG_FILE"
done