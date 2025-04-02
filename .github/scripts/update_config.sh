#! /bin/sh

CONFIG_FILES=(./test/config/test.erigon.seq.config.yaml)

for CONFIG_FILE in "${CONFIG_FILES[@]}"; do
#  cat "$CONFIG_FILE"
  echo "Updating $CONFIG_FILE..."
  sed -i '/^zkevm.executor-urls:/d' "$CONFIG_FILE"
  sed -i '/^zkevm.executor-strict:/d' "$CONFIG_FILE"
  sed -i '/^zkevm.witness-full:/d' "$CONFIG_FILE"
  sed -i '/^zkevm.executor-mock:/d' "$CONFIG_FILE"

  echo 'zkevm.executor-urls: "xlayer-executor:50071"' >> "$CONFIG_FILE"
  echo 'zkevm.executor-strict: true' >> "$CONFIG_FILE"
  echo 'zkevm.witness-full: false' >> "$CONFIG_FILE"
  echo 'zkevm.executor-mock: false' >> "$CONFIG_FILE"


  echo "Finished updating $CONFIG_FILE."
#  cat "$CONFIG_FILE"
done