package utils

import "time"

const (
	DEFAULT_DESTINATION_DIR          = ".lrp"
	DEFAULT_SOURCE_MAINNET_DATA_PATH = "mainnet"
	DEFAULT_EXTERNAL_DATASTREAM_PATH = "mainnet/seq/data-stream"
	DEFAULT_SAMPLE_INTERVAL          = 10 * time.Second
	MIN_SMAPLE_INTERVAL              = time.Second
	DEFAULT_CHAINDATA_LIMIT          = 100 * 1024 * 1024 * 1024

	REPO_NAME       = "xlayer-erigon"
	LRP_CONFIG_FILE = "lrp.config.yaml"
	UNWIND_LOG      = "unwind.log"
	REPLAY_LOG      = "replay.log"
	UNWOUND_REPO    = "unwound-repo"

	// commands
	LRP_CONFIG                 = "lrp-config"
	LRP_MAINNET_UNWIND         = "lrp-mainnet-unwind"
	LRP_MAINNET_REPLAY         = "lrp-mainnet-replay"
	LRP_MAINNET_REPLAY_VMTOUCH = "lrp-mainnet-replay-vmtouch"
	LRP_STOP                   = "lrp-stop"
	LRP_CLEAN                  = "lrp-clean"
	LRP_MAINNET_REPLAY_PAUSE   = "lrp-mainnet-replay-pause"

	// stop sign
	REPLAY_STOP_SIGN = "Resequencing completed"
)
